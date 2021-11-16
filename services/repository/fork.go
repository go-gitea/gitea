// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
)

// ForkRepository forks a repository
func ForkRepository(doer, owner *models.User, opts models.ForkRepoOptions) (_ *models.Repository, err error) {
	forkedRepo, err := opts.BaseRepo.GetUserFork(owner.ID)
	if err != nil {
		return nil, err
	}
	if forkedRepo != nil {
		return nil, models.ErrForkAlreadyExist{
			Uname:    owner.Name,
			RepoName: opts.BaseRepo.FullName(),
			ForkName: forkedRepo.FullName(),
		}
	}

	repo := &models.Repository{
		OwnerID:       owner.ID,
		Owner:         owner,
		OwnerName:     owner.Name,
		Name:          opts.Name,
		LowerName:     strings.ToLower(opts.Name),
		Description:   opts.Description,
		DefaultBranch: opts.BaseRepo.DefaultBranch,
		IsPrivate:     opts.BaseRepo.IsPrivate || opts.BaseRepo.Owner.Visibility == structs.VisibleTypePrivate,
		IsEmpty:       opts.BaseRepo.IsEmpty,
		IsFork:        true,
		ForkID:        opts.BaseRepo.ID,
	}

	oldRepoPath := opts.BaseRepo.RepoPath()

	needsRollback := false
	rollbackFn := func() {
		if !needsRollback {
			return
		}

		repoPath := models.RepoPath(owner.Name, repo.Name)

		if exists, _ := util.IsExist(repoPath); !exists {
			return
		}

		// As the transaction will be failed and hence database changes will be destroyed we only need
		// to delete the related repository on the filesystem
		if errDelete := util.RemoveAll(repoPath); errDelete != nil {
			log.Error("Failed to remove fork repo")
		}
	}

	needsRollbackInPanic := true
	defer func() {
		panicErr := recover()
		if panicErr == nil {
			return
		}

		if needsRollbackInPanic {
			rollbackFn()
		}
		panic(panicErr)
	}()

	err = db.WithTx(func(ctx context.Context) error {
		if err = models.CreateRepository(ctx, doer, owner, repo, false); err != nil {
			return err
		}

		if err = models.IncrementRepoForkNum(ctx, opts.BaseRepo.ID); err != nil {
			return err
		}

		// copy lfs files failure should not be ignored
		if err = models.CopyLFS(ctx, repo, opts.BaseRepo); err != nil {
			return err
		}

		needsRollback = true

		repoPath := models.RepoPath(owner.Name, repo.Name)
		if stdout, err := git.NewCommandContext(ctx,
			"clone", "--bare", oldRepoPath, repoPath).
			SetDescription(fmt.Sprintf("ForkRepository(git clone): %s to %s", opts.BaseRepo.FullName(), repo.FullName())).
			RunInDirTimeout(10*time.Minute, ""); err != nil {
			log.Error("Fork Repository (git clone) Failed for %v (from %v):\nStdout: %s\nError: %v", repo, opts.BaseRepo, stdout, err)
			return fmt.Errorf("git clone: %v", err)
		}

		if err := repo.CheckDaemonExportOK(ctx); err != nil {
			return fmt.Errorf("checkDaemonExportOK: %v", err)
		}

		if stdout, err := git.NewCommandContext(ctx, "update-server-info").
			SetDescription(fmt.Sprintf("ForkRepository(git update-server-info): %s", repo.FullName())).
			RunInDir(repoPath); err != nil {
			log.Error("Fork Repository (git update-server-info) failed for %v:\nStdout: %s\nError: %v", repo, stdout, err)
			return fmt.Errorf("git update-server-info: %v", err)
		}

		if err = repo_module.CreateDelegateHooks(repoPath); err != nil {
			return fmt.Errorf("createDelegateHooks: %v", err)
		}
		return nil
	})
	needsRollbackInPanic = false
	if err != nil {
		rollbackFn()
		return nil, err
	}

	// even if below operations failed, it could be ignored. And they will be retried
	ctx := db.DefaultContext
	if err := repo.UpdateSize(ctx); err != nil {
		log.Error("Failed to update size for repository: %v", err)
	}
	if err := models.CopyLanguageStat(opts.BaseRepo, repo); err != nil {
		log.Error("Copy language stat from oldRepo failed")
	}

	notification.NotifyForkRepository(doer, opts.BaseRepo, repo)

	return repo, nil
}

// ConvertForkToNormalRepository convert the provided repo from a forked repo to normal repo
func ConvertForkToNormalRepository(repo *models.Repository) error {
	err := db.WithTx(func(ctx context.Context) error {
		repo, err := models.GetRepositoryByIDCtx(ctx, repo.ID)
		if err != nil {
			return err
		}

		if !repo.IsFork {
			return nil
		}

		if err := models.DecrementRepoForkNum(ctx, repo.ForkID); err != nil {
			log.Error("Unable to decrement repo fork num for old root repo %d of repository %-v whilst converting from fork. Error: %v", repo.ForkID, repo, err)
			return err
		}

		repo.IsFork = false
		repo.ForkID = 0

		if err := models.UpdateRepositoryCtx(ctx, repo, false); err != nil {
			log.Error("Unable to update repository %-v whilst converting from fork. Error: %v", repo, err)
			return err
		}

		return nil
	})

	return err
}
