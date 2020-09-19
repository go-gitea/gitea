// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"github.com/unknwon/com"
)

// AdoptRepository adopts a repository for the user/organization.
func AdoptRepository(doer, u *models.User, opts models.CreateRepoOptions) (*models.Repository, error) {
	if !doer.IsAdmin && !u.CanCreateRepo() {
		return nil, models.ErrReachLimitOfRepo{
			Limit: u.MaxRepoCreation,
		}
	}

	if len(opts.DefaultBranch) == 0 {
		opts.DefaultBranch = setting.Repository.DefaultBranch
	}

	repo := &models.Repository{
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
		CloseIssuesViaCommitInAnyBranch: setting.Repository.DefaultCloseIssuesViaCommitsInAnyBranch,
		Status:                          opts.Status,
		IsEmpty:                         !opts.AutoInit,
	}

	if err := models.WithTx(func(ctx models.DBContext) error {
		repoPath := models.RepoPath(u.Name, repo.Name)
		if !com.IsExist(repoPath) {
			return models.ErrRepoNotExist{
				OwnerName: u.Name,
				Name:      repo.Name,
			}
		}

		if err := models.CreateRepository(ctx, doer, u, repo, true); err != nil {
			return err
		}
		if err := adoptRepository(ctx, repoPath, doer, repo, opts); err != nil {
			return fmt.Errorf("createDelegateHooks: %v", err)
		}

		// Initialize Issue Labels if selected
		if len(opts.IssueLabels) > 0 {
			if err := models.InitializeLabels(ctx, repo.ID, opts.IssueLabels, false); err != nil {
				return fmt.Errorf("InitializeLabels: %v", err)
			}
		}

		if stdout, err := git.NewCommand("update-server-info").
			SetDescription(fmt.Sprintf("CreateRepository(git update-server-info): %s", repoPath)).
			RunInDir(repoPath); err != nil {
			log.Error("CreateRepository(git update-server-info) in %v: Stdout: %s\nError: %v", repo, stdout, err)
			return fmt.Errorf("CreateRepository(git update-server-info): %v", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return repo, nil
}

// DeleteUnadoptedRepository deletes unadopted repository files from the filesystem
func DeleteUnadoptedRepository(doer, u *models.User, repoName string) error {
	if err := models.IsUsableRepoName(repoName); err != nil {
		return err
	}

	repoPath := models.RepoPath(u.Name, repoName)
	if !com.IsExist(repoPath) {
		return models.ErrRepoNotExist{
			OwnerName: u.Name,
			Name:      repoName,
		}
	}

	if exist, err := models.IsRepositoryExist(u, repoName); err != nil {
		return err
	} else if exist {
		return models.ErrRepoAlreadyExist{
			Uname: u.Name,
			Name:  repoName,
		}
	}

	return util.RemoveAll(repoPath)
}
