// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"unicode/utf8"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
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
		if _, err := GetLabelTemplateFile(opts.IssueLabels); err != nil {
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
		IsMirror:                        opts.IsMirror,
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
			if err = InitializeLabels(ctx, repo.ID, opts.IssueLabels, false); err != nil {
				rollbackRepo = repo
				rollbackRepo.OwnerID = u.ID
				return fmt.Errorf("InitializeLabels: %v", err)
			}
		}

		if err := CheckDaemonExportOK(ctx, repo); err != nil {
			return fmt.Errorf("checkDaemonExportOK: %v", err)
		}

		if stdout, _, err := git.NewCommand(ctx, "update-server-info").
			SetDescription(fmt.Sprintf("CreateRepository(git update-server-info): %s", repoPath)).
			RunStdString(&git.RunOpts{Dir: repoPath}); err != nil {
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

// UpdateRepoSize updates the repository size, calculating it using util.GetDirectorySize
func UpdateRepoSize(ctx context.Context, repo *repo_model.Repository) error {
	size, err := util.GetDirectorySize(repo.RepoPath())
	if err != nil {
		return fmt.Errorf("updateSize: %v", err)
	}

	lfsSize, err := git_model.GetRepoLFSSize(ctx, repo.ID)
	if err != nil {
		return fmt.Errorf("updateSize: GetLFSMetaObjects: %v", err)
	}

	return repo_model.UpdateRepoSize(ctx, repo.ID, size+lfsSize)
}

// CheckDaemonExportOK creates/removes git-daemon-export-ok for git-daemon...
func CheckDaemonExportOK(ctx context.Context, repo *repo_model.Repository) error {
	if err := repo.GetOwner(ctx); err != nil {
		return err
	}

	// Create/Remove git-daemon-export-ok for git-daemon...
	daemonExportFile := path.Join(repo.RepoPath(), `git-daemon-export-ok`)

	isExist, err := util.IsExist(daemonExportFile)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", daemonExportFile, err)
		return err
	}

	isPublic := !repo.IsPrivate && repo.Owner.Visibility == api.VisibleTypePublic
	if !isPublic && isExist {
		if err = util.Remove(daemonExportFile); err != nil {
			log.Error("Failed to remove %s: %v", daemonExportFile, err)
		}
	} else if isPublic && !isExist {
		if f, err := os.Create(daemonExportFile); err != nil {
			log.Error("Failed to create %s: %v", daemonExportFile, err)
		} else {
			f.Close()
		}
	}

	return nil
}

// UpdateRepository updates a repository with db context
func UpdateRepository(ctx context.Context, repo *repo_model.Repository, visibilityChanged bool) (err error) {
	repo.LowerName = strings.ToLower(repo.Name)

	if utf8.RuneCountInString(repo.Description) > 255 {
		repo.Description = string([]rune(repo.Description)[:255])
	}
	if utf8.RuneCountInString(repo.Website) > 255 {
		repo.Website = string([]rune(repo.Website)[:255])
	}

	e := db.GetEngine(ctx)

	if _, err = e.ID(repo.ID).AllCols().Update(repo); err != nil {
		return fmt.Errorf("update: %v", err)
	}

	if err = UpdateRepoSize(ctx, repo); err != nil {
		log.Error("Failed to update size for repository: %v", err)
	}

	if visibilityChanged {
		if err = repo.GetOwner(ctx); err != nil {
			return fmt.Errorf("getOwner: %v", err)
		}
		if repo.Owner.IsOrganization() {
			// Organization repository need to recalculate access table when visibility is changed.
			if err = access_model.RecalculateTeamAccesses(ctx, repo, 0); err != nil {
				return fmt.Errorf("recalculateTeamAccesses: %v", err)
			}
		}

		// If repo has become private, we need to set its actions to private.
		if repo.IsPrivate {
			_, err = e.Where("repo_id = ?", repo.ID).Cols("is_private").Update(&models.Action{
				IsPrivate: true,
			})
			if err != nil {
				return err
			}
		}

		// Create/Remove git-daemon-export-ok for git-daemon...
		if err := CheckDaemonExportOK(ctx, repo); err != nil {
			return err
		}

		forkRepos, err := repo_model.GetRepositoriesByForkID(ctx, repo.ID)
		if err != nil {
			return fmt.Errorf("getRepositoriesByForkID: %v", err)
		}
		for i := range forkRepos {
			forkRepos[i].IsPrivate = repo.IsPrivate || repo.Owner.Visibility == api.VisibleTypePrivate
			if err = UpdateRepository(ctx, forkRepos[i], true); err != nil {
				return fmt.Errorf("updateRepository[%d]: %v", forkRepos[i].ID, err)
			}
		}
	}

	return nil
}
