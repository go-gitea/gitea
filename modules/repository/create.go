// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// CreateRepository creates a repository for the user/organization.
func CreateRepository(doer, u *user_model.User, opts models.CreateRepoOptions) (*repo_model.Repository, error) {
	if !doer.IsAdmin && !u.CanCreateRepo() {
		return nil, repo_model.ErrReachLimitOfRepo{
			Limit: u.MaxRepoCreation,
		}
	}

	if len(opts.DefaultBranch) == 0 {
		opts.DefaultBranch = setting.Repository.DefaultBranch
	}

	// Check if label template exist
	if len(opts.IssueLabels) > 0 {
		if _, err := models.GetLabelTemplateFile(opts.IssueLabels); err != nil {
			return nil, err
		}
	}

	repo := &repo_model.Repository{
		OwnerID:                         u.ID,
		Owner:                           u,
		OwnerName:                       u.Name,
		Name:                            opts.Name,
		LowerName:                       strings.ToLower(opts.Name),
		Description:                     opts.Description,
		OriginalURL:                     opts.OriginalURL,
		OriginalServiceType:             opts.GitServiceType,
		IsPrivate:                       opts.IsPrivate,
		IsFsckEnabled:                   !opts.IsMirror,
		IsTemplate:                      opts.IsTemplate,
		CloseIssuesViaCommitInAnyBranch: setting.Repository.DefaultCloseIssuesViaCommitsInAnyBranch,
		Status:                          opts.Status,
		IsEmpty:                         !opts.AutoInit,
		TrustModel:                      opts.TrustModel,
	}

	var rollbackRepo *repo_model.Repository

	if err := db.WithTx(func(ctx context.Context) error {
		if err := models.CreateRepository(ctx, doer, u, repo, false); err != nil {
			return err
		}

		// No need for init mirror.
		if opts.IsMirror {
			return nil
		}

		repoPath := repo_model.RepoPath(u.Name, repo.Name)
		isExist, err := util.IsExist(repoPath)
		if err != nil {
			log.Error("Unable to check if %s exists. Error: %v", repoPath, err)
			return err
		}
		if isExist {
			// repo already exists - We have two or three options.
			// 1. We fail stating that the directory exists
			// 2. We create the db repository to go with this data and adopt the git repo
			// 3. We delete it and start afresh
			//
			// Previously Gitea would just delete and start afresh - this was naughty.
			// So we will now fail and delegate to other functionality to adopt or delete
			log.Error("Files already exist in %s and we are not going to adopt or delete.", repoPath)
			return repo_model.ErrRepoFilesAlreadyExist{
				Uname: u.Name,
				Name:  repo.Name,
			}
		}

		if err = initRepository(ctx, repoPath, doer, repo, opts); err != nil {
			if err2 := util.RemoveAll(repoPath); err2 != nil {
				log.Error("initRepository: %v", err)
				return fmt.Errorf(
					"delete repo directory %s/%s failed(2): %v", u.Name, repo.Name, err2)
			}
			return fmt.Errorf("initRepository: %v", err)
		}

		// Initialize Issue Labels if selected
		if len(opts.IssueLabels) > 0 {
			if err = models.InitializeLabels(ctx, repo.ID, opts.IssueLabels, false); err != nil {
				rollbackRepo = repo
				rollbackRepo.OwnerID = u.ID
				return fmt.Errorf("InitializeLabels: %v", err)
			}
		}

		if err := models.CheckDaemonExportOK(ctx, repo); err != nil {
			return fmt.Errorf("checkDaemonExportOK: %v", err)
		}

		if stdout, err := git.NewCommand(ctx, "update-server-info").
			SetDescription(fmt.Sprintf("CreateRepository(git update-server-info): %s", repoPath)).
			RunInDir(repoPath); err != nil {
			log.Error("CreateRepository(git update-server-info) in %v: Stdout: %s\nError: %v", repo, stdout, err)
			rollbackRepo = repo
			rollbackRepo.OwnerID = u.ID
			return fmt.Errorf("CreateRepository(git update-server-info): %v", err)
		}
		return nil
	}); err != nil {
		if rollbackRepo != nil {
			if errDelete := models.DeleteRepository(doer, rollbackRepo.OwnerID, rollbackRepo.ID); errDelete != nil {
				log.Error("Rollback deleteRepository: %v", errDelete)
			}
		}

		return nil, err
	}

	return repo, nil
}
