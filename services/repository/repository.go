// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/graceful"
	issue_indexer "code.gitea.io/gitea/modules/indexer/issues"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	notify_service "code.gitea.io/gitea/services/notify"
	pull_service "code.gitea.io/gitea/services/pull"
)

// WebSearchRepository represents a repository returned by web search
type WebSearchRepository struct {
	Repository               *structs.Repository `json:"repository"`
	LatestCommitStatus       *git.CommitStatus   `json:"latest_commit_status"`
	LocaleLatestCommitStatus string              `json:"locale_latest_commit_status"`
}

// WebSearchResults results of a successful web search
type WebSearchResults struct {
	OK   bool                   `json:"ok"`
	Data []*WebSearchRepository `json:"data"`
}

// CreateRepository creates a repository for the user/organization.
func CreateRepository(ctx context.Context, doer, owner *user_model.User, opts CreateRepoOptions) (*repo_model.Repository, error) {
	repo, err := CreateRepositoryDirectly(ctx, doer, owner, opts, true)
	if err != nil {
		// No need to rollback here we should do this in CreateRepository...
		return nil, err
	}

	notify_service.CreateRepository(ctx, doer, owner, repo)

	return repo, nil
}

// DeleteRepository deletes a repository for a user or organization.
func DeleteRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, notify bool) error {
	if err := pull_service.CloseRepoBranchesPulls(ctx, doer, repo); err != nil {
		log.Error("CloseRepoBranchesPulls failed: %v", err)
	}

	if notify {
		// If the repo itself has webhooks, we need to trigger them before deleting it...
		notify_service.DeleteRepository(ctx, doer, repo)
	}

	return DeleteRepositoryDirectly(ctx, repo.ID)
}

// PushCreateRepo creates a repository when a new repository is pushed to an appropriate namespace
func PushCreateRepo(ctx context.Context, authUser, owner *user_model.User, repoName string) (*repo_model.Repository, error) {
	if !authUser.IsAdmin {
		if owner.IsOrganization() {
			if ok, err := organization.CanCreateOrgRepo(ctx, owner.ID, authUser.ID); err != nil {
				return nil, err
			} else if !ok {
				return nil, errors.New("cannot push-create repository for org")
			}
		} else if authUser.ID != owner.ID {
			return nil, errors.New("cannot push-create repository for another user")
		}
	}

	repo, err := CreateRepository(ctx, authUser, owner, CreateRepoOptions{
		Name:      repoName,
		IsPrivate: setting.Repository.DefaultPushCreatePrivate || setting.Repository.ForcePrivate,
	})
	if err != nil {
		return nil, err
	}

	return repo, nil
}

// Init start repository service
func Init(ctx context.Context) error {
	licenseUpdaterQueue = queue.CreateUniqueQueue(graceful.GetManager().ShutdownContext(), "repo_license_updater", repoLicenseUpdater)
	if licenseUpdaterQueue == nil {
		return errors.New("unable to create repo_license_updater queue")
	}
	go graceful.GetManager().RunWithCancel(licenseUpdaterQueue)

	if err := repo_module.LoadRepoConfig(); err != nil {
		return err
	}
	if err := initPushQueue(); err != nil {
		return err
	}
	return initBranchSyncQueue(graceful.GetManager().ShutdownContext())
}

// UpdateRepository updates a repository
func UpdateRepository(ctx context.Context, repo *repo_model.Repository, visibilityChanged bool) (err error) {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err = updateRepository(ctx, repo, visibilityChanged); err != nil {
			return fmt.Errorf("updateRepository: %w", err)
		}
		return nil
	})
}

func MakeRepoPublic(ctx context.Context, repo *repo_model.Repository) (err error) {
	return db.WithTx(ctx, func(ctx context.Context) error {
		repo.IsPrivate = false
		if err := repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo, "is_private"); err != nil {
			return err
		}

		if err = repo.LoadOwner(ctx); err != nil {
			return fmt.Errorf("LoadOwner: %w", err)
		}
		if repo.Owner.IsOrganization() {
			// Organization repository need to recalculate access table when visibility is changed.
			if err = access_model.RecalculateTeamAccesses(ctx, repo, 0); err != nil {
				return fmt.Errorf("recalculateTeamAccesses: %w", err)
			}
		}

		// Create/Remove git-daemon-export-ok for git-daemon...
		if err := CheckDaemonExportOK(ctx, repo); err != nil {
			return err
		}

		forkRepos, err := repo_model.GetRepositoriesByForkID(ctx, repo.ID)
		if err != nil {
			return fmt.Errorf("getRepositoriesByForkID: %w", err)
		}

		if repo.Owner.Visibility != structs.VisibleTypePrivate {
			for i := range forkRepos {
				if err = MakeRepoPublic(ctx, forkRepos[i]); err != nil {
					return fmt.Errorf("MakeRepoPublic[%d]: %w", forkRepos[i].ID, err)
				}
			}
		}

		// If visibility is changed, we need to update the issue indexer.
		// Since the data in the issue indexer have field to indicate if the repo is public or not.
		issue_indexer.UpdateRepoIndexer(ctx, repo.ID)

		return nil
	})
}

func MakeRepoPrivate(ctx context.Context, repo *repo_model.Repository) (err error) {
	return db.WithTx(ctx, func(ctx context.Context) error {
		repo.IsPrivate = true
		if err := repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo, "is_private"); err != nil {
			return err
		}

		if err = repo.LoadOwner(ctx); err != nil {
			return fmt.Errorf("LoadOwner: %w", err)
		}
		if repo.Owner.IsOrganization() {
			// Organization repository need to recalculate access table when visibility is changed.
			if err = access_model.RecalculateTeamAccesses(ctx, repo, 0); err != nil {
				return fmt.Errorf("recalculateTeamAccesses: %w", err)
			}
		}

		// If repo has become private, we need to set its actions to private.
		_, err = db.GetEngine(ctx).Where("repo_id = ?", repo.ID).Cols("is_private").Update(&activities_model.Action{
			IsPrivate: true,
		})
		if err != nil {
			return err
		}

		if err = repo_model.ClearRepoStars(ctx, repo.ID); err != nil {
			return err
		}

		// Create/Remove git-daemon-export-ok for git-daemon...
		if err := CheckDaemonExportOK(ctx, repo); err != nil {
			return err
		}

		forkRepos, err := repo_model.GetRepositoriesByForkID(ctx, repo.ID)
		if err != nil {
			return fmt.Errorf("getRepositoriesByForkID: %w", err)
		}
		for i := range forkRepos {
			if err = MakeRepoPrivate(ctx, forkRepos[i]); err != nil {
				return fmt.Errorf("MakeRepoPrivate[%d]: %w", forkRepos[i].ID, err)
			}
		}

		// If visibility is changed, we need to update the issue indexer.
		// Since the data in the issue indexer have field to indicate if the repo is public or not.
		issue_indexer.UpdateRepoIndexer(ctx, repo.ID)

		return nil
	})
}

// LinkedRepository returns the linked repo if any
func LinkedRepository(ctx context.Context, a *repo_model.Attachment) (*repo_model.Repository, unit.Type, error) {
	if a.IssueID != 0 {
		iss, err := issues_model.GetIssueByID(ctx, a.IssueID)
		if err != nil {
			return nil, unit.TypeIssues, err
		}
		repo, err := repo_model.GetRepositoryByID(ctx, iss.RepoID)
		unitType := unit.TypeIssues
		if iss.IsPull {
			unitType = unit.TypePullRequests
		}
		return repo, unitType, err
	} else if a.ReleaseID != 0 {
		rel, err := repo_model.GetReleaseByID(ctx, a.ReleaseID)
		if err != nil {
			return nil, unit.TypeReleases, err
		}
		repo, err := repo_model.GetRepositoryByID(ctx, rel.RepoID)
		return repo, unit.TypeReleases, err
	}
	return nil, -1, nil
}

// CheckDaemonExportOK creates/removes git-daemon-export-ok for git-daemon...
func CheckDaemonExportOK(ctx context.Context, repo *repo_model.Repository) error {
	if err := repo.LoadOwner(ctx); err != nil {
		return err
	}

	// Create/Remove git-daemon-export-ok for git-daemon...
	daemonExportFile := filepath.Join(repo.RepoPath(), `git-daemon-export-ok`)

	isExist, err := util.IsExist(daemonExportFile)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", daemonExportFile, err)
		return err
	}

	isPublic := !repo.IsPrivate && repo.Owner.Visibility == structs.VisibleTypePublic
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

// updateRepository updates a repository with db context
func updateRepository(ctx context.Context, repo *repo_model.Repository, visibilityChanged bool) (err error) {
	repo.LowerName = strings.ToLower(repo.Name)

	e := db.GetEngine(ctx)

	if _, err = e.ID(repo.ID).NoAutoTime().AllCols().Update(repo); err != nil {
		return fmt.Errorf("update: %w", err)
	}

	if err = repo_module.UpdateRepoSize(ctx, repo); err != nil {
		log.Error("Failed to update size for repository: %v", err)
	}

	if visibilityChanged {
		if err = repo.LoadOwner(ctx); err != nil {
			return fmt.Errorf("LoadOwner: %w", err)
		}
		if repo.Owner.IsOrganization() {
			// Organization repository need to recalculate access table when visibility is changed.
			if err = access_model.RecalculateTeamAccesses(ctx, repo, 0); err != nil {
				return fmt.Errorf("recalculateTeamAccesses: %w", err)
			}
		}

		// If repo has become private, we need to set its actions to private.
		if repo.IsPrivate {
			_, err = e.Where("repo_id = ?", repo.ID).Cols("is_private").Update(&activities_model.Action{
				IsPrivate: true,
			})
			if err != nil {
				return err
			}

			if err = repo_model.ClearRepoStars(ctx, repo.ID); err != nil {
				return err
			}
		}

		// Create/Remove git-daemon-export-ok for git-daemon...
		if err := CheckDaemonExportOK(ctx, repo); err != nil {
			return err
		}

		forkRepos, err := repo_model.GetRepositoriesByForkID(ctx, repo.ID)
		if err != nil {
			return fmt.Errorf("getRepositoriesByForkID: %w", err)
		}
		for i := range forkRepos {
			forkRepos[i].IsPrivate = repo.IsPrivate || repo.Owner.Visibility == structs.VisibleTypePrivate
			if err = updateRepository(ctx, forkRepos[i], true); err != nil {
				return fmt.Errorf("updateRepository[%d]: %w", forkRepos[i].ID, err)
			}
		}

		// If visibility is changed, we need to update the issue indexer.
		// Since the data in the issue indexer have field to indicate if the repo is public or not.
		issue_indexer.UpdateRepoIndexer(ctx, repo.ID)
	}

	return nil
}
