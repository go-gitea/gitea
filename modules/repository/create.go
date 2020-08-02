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
	"github.com/unknwon/com"
)

// CreateRepository creates a repository for the user/organization.
func CreateRepository(doer, u *models.User, opts models.CreateRepoOptions) (*models.Repository, error) {
	if !doer.IsAdmin && !u.CanCreateRepo() {
		return nil, models.ErrReachLimitOfRepo{
			Limit: u.MaxRepoCreation,
		}
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

	overwriteOrAdopt := (!opts.IsMirror && opts.AdoptPreExisting && setting.Repository.AllowAdoptionOfUnadoptedRepositories) ||
		(opts.OverwritePreExisting && setting.Repository.AllowOverwriteOfUnadoptedRepositories)

	repoPath := models.RepoPath(u.Name, repo.Name)
	if !overwriteOrAdopt && com.IsExist(repoPath) {
		log.Error("Files already exist in %s and we are not going to adopt or delete.", repoPath)
		return nil, models.ErrRepoFilesAlreadyExist{
			Uname: u.Name,
			Name:  opts.Name,
		}
	}

	if err := models.WithTx(func(ctx models.DBContext) error {
		if err := models.CreateRepository(ctx, doer, u, repo); err != nil {
			return err
		}

		// No need for init mirror.
		if !opts.IsMirror {
			// repo already exists - We have two or three options.
			// 1. We fail stating that the directory exists
			// 2. We create the db repository to go with this data and adopt the git repo
			// 3. We delete it and start afresh
			//
			// Previously Gitea would just delete and start afresh - this was naughty.
			shouldInit := true
			if com.IsExist(repoPath) {
				if opts.AdoptPreExisting {
					shouldInit = false
					if err := adoptRepository(ctx, repoPath, doer, repo, opts); err != nil {
						return fmt.Errorf("createDelegateHooks: %v", err)
					}
				} else if opts.OverwritePreExisting {
					log.Warn("An already existing repository was deleted at %s", repoPath)
					if err := os.RemoveAll(repoPath); err != nil {
						log.Error("Unable to remove already existing repository at %s: Error: %v", repoPath, err)
						return fmt.Errorf(
							"unable to delete repo directory %s/%s: %v", u.Name, repo.Name, err)
					}
				} else {
					log.Error("Files already exist in %s and not going to adopt or delete.", repoPath)
					return fmt.Errorf("data already exists on the filesystem for %s/%s. You will need to adopt these or delete these explicitly", u.Name, repo.Name)
				}
			}

			if shouldInit {
				if err := initRepository(ctx, repoPath, doer, repo, opts); err != nil {
					if err2 := os.RemoveAll(repoPath); err2 != nil {
						log.Error("initRepository: %v", err)
						return fmt.Errorf(
							"delete repo directory %s/%s failed(2): %v", u.Name, repo.Name, err2)
					}
					return fmt.Errorf("initRepository: %v", err)
				}
			}

			// Initialize Issue Labels if selected
			if len(opts.IssueLabels) > 0 {
				if err := models.InitializeLabels(ctx, repo.ID, opts.IssueLabels, false); err != nil {
					if shouldInit {
						if errDelete := models.DeleteRepository(doer, u.ID, repo.ID); errDelete != nil {
							log.Error("Rollback deleteRepository: %v", errDelete)
						}
					}
					return fmt.Errorf("InitializeLabels: %v", err)
				}
			}

			if stdout, err := git.NewCommand("update-server-info").
				SetDescription(fmt.Sprintf("CreateRepository(git update-server-info): %s", repoPath)).
				RunInDir(repoPath); err != nil {
				log.Error("CreateRepository(git update-server-info) in %v: Stdout: %s\nError: %v", repo, stdout, err)
				if shouldInit {
					if errDelete := models.DeleteRepository(doer, u.ID, repo.ID); errDelete != nil {
						log.Error("Rollback deleteRepository: %v", errDelete)
					}
				}
				return fmt.Errorf("CreateRepository(git update-server-info): %v", err)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return repo, nil
}
