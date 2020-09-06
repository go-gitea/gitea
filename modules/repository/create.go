// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"fmt"
	"os"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// CreateRepository creates a repository for the user/organization.
func CreateRepository(doer, u *models.User, opts models.CreateRepoOptions) (_ *models.Repository, err error) {
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

	err = models.WithTx(func(ctx models.DBContext) error {
		if err = models.CreateRepository(ctx, doer, u, repo); err != nil {
			return err
		}

		// No need for init mirror.
		if !opts.IsMirror {
			repoPath := models.RepoPath(u.Name, repo.Name)
			if err = initRepository(ctx, repoPath, doer, repo, opts); err != nil {
				if err2 := os.RemoveAll(repoPath); err2 != nil {
					log.Error("initRepository: %v", err)
					return fmt.Errorf(
						"delete repo directory %s/%s failed(2): %v", u.Name, repo.Name, err2)
				}
				return fmt.Errorf("initRepository: %v", err)
			}

			// Initialize Issue Labels if selected
			if len(opts.IssueLabels) > 0 {
				if err = models.InitializeLabels(ctx, repo.ID, opts.IssueLabels, false); err != nil {
					return fmt.Errorf("InitializeLabels: %v", err)
				}
			}

			if stdout, err := git.NewCommand("update-server-info").
				SetDescription(fmt.Sprintf("CreateRepository(git update-server-info): %s", repoPath)).
				RunInDir(repoPath); err != nil {
				log.Error("CreateRepository(git update-server-info) in %v: Stdout: %s\nError: %v", repo, stdout, err)
				return fmt.Errorf("CreateRepository(git update-server-info): %v", err)
			}
		}
		return nil
	})

	return repo, err
}
