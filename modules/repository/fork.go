// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"
)

// ForkRepository forks a repository
func ForkRepository(doer, owner *models.User, oldRepo *models.Repository, name, desc string) (_ *models.Repository, err error) {
	forkedRepo, err := oldRepo.GetUserFork(owner.ID)
	if err != nil {
		return nil, err
	}
	if forkedRepo != nil {
		return nil, models.ErrForkAlreadyExist{
			Uname:    owner.Name,
			RepoName: oldRepo.FullName(),
			ForkName: forkedRepo.FullName(),
		}
	}

	repo := &models.Repository{
		OwnerID:       owner.ID,
		Owner:         owner,
		OwnerName:     owner.Name,
		Name:          name,
		LowerName:     strings.ToLower(name),
		Description:   desc,
		DefaultBranch: oldRepo.DefaultBranch,
		IsPrivate:     oldRepo.IsPrivate || oldRepo.Owner.Visibility == structs.VisibleTypePrivate,
		IsEmpty:       oldRepo.IsEmpty,
		IsFork:        true,
		ForkID:        oldRepo.ID,
	}

	oldRepoPath := oldRepo.RepoPath()

	err = models.WithTx(func(ctx models.DBContext) error {
		if err = models.CreateRepository(ctx, doer, owner, repo, false); err != nil {
			return err
		}

		rollbackRemoveFn := func() {
			if repo.ID == 0 {
				return
			}
			if errDelete := models.DeleteRepository(doer, owner.ID, repo.ID); errDelete != nil {
				log.Error("Rollback deleteRepository: %v", errDelete)
			}
		}

		if err = models.IncrementRepoForkNum(ctx, oldRepo.ID); err != nil {
			rollbackRemoveFn()
			return err
		}

		repoPath := models.RepoPath(owner.Name, repo.Name)
		if stdout, err := git.NewCommand(
			"clone", "--bare", oldRepoPath, repoPath).
			SetDescription(fmt.Sprintf("ForkRepository(git clone): %s to %s", oldRepo.FullName(), repo.FullName())).
			RunInDirTimeout(10*time.Minute, ""); err != nil {
			log.Error("Fork Repository (git clone) Failed for %v (from %v):\nStdout: %s\nError: %v", repo, oldRepo, stdout, err)
			rollbackRemoveFn()
			return fmt.Errorf("git clone: %v", err)
		}

		if stdout, err := git.NewCommand("update-server-info").
			SetDescription(fmt.Sprintf("ForkRepository(git update-server-info): %s", repo.FullName())).
			RunInDir(repoPath); err != nil {
			log.Error("Fork Repository (git update-server-info) failed for %v:\nStdout: %s\nError: %v", repo, stdout, err)
			rollbackRemoveFn()
			return fmt.Errorf("git update-server-info: %v", err)
		}

		if err = createDelegateHooks(repoPath); err != nil {
			rollbackRemoveFn()
			return fmt.Errorf("createDelegateHooks: %v", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	ctx := models.DefaultDBContext()
	if err = repo.UpdateSize(ctx); err != nil {
		log.Error("Failed to update size for repository: %v", err)
	}
	if err := models.CopyLanguageStat(oldRepo, repo); err != nil {
		log.Error("Copy language stat from oldRepo failed")
	}

	if err := models.CopyLFS(ctx, repo, oldRepo); err != nil {
		if errDelete := models.DeleteRepository(doer, owner.ID, repo.ID); errDelete != nil {
			log.Error("Rollback deleteRepository: %v", errDelete)
		}
		return nil, err
	}
	return repo, nil
}
