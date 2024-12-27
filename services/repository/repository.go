// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	system_model "code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
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
	repo, err := CreateRepositoryDirectly(ctx, doer, owner, opts)
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

	if err := DeleteRepositoryDirectly(ctx, doer, repo.ID); err != nil {
		return err
	}

	return packages_model.UnlinkRepositoryFromAllPackages(ctx, repo.ID)
}

// PushCreateRepo creates a repository when a new repository is pushed to an appropriate namespace
func PushCreateRepo(ctx context.Context, authUser, owner *user_model.User, repoName string) (*repo_model.Repository, error) {
	if !authUser.IsAdmin {
		if owner.IsOrganization() {
			if ok, err := organization.CanCreateOrgRepo(ctx, owner.ID, authUser.ID); err != nil {
				return nil, err
			} else if !ok {
				return nil, fmt.Errorf("cannot push-create repository for org")
			}
		} else if authUser.ID != owner.ID {
			return nil, fmt.Errorf("cannot push-create repository for another user")
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
		return fmt.Errorf("unable to create repo_license_updater queue")
	}
	go graceful.GetManager().RunWithCancel(licenseUpdaterQueue)

	if err := repo_module.LoadRepoConfig(); err != nil {
		return err
	}
	system_model.RemoveAllWithNotice(ctx, "Clean up temporary repository uploads", setting.Repository.Upload.TempPath)
	system_model.RemoveAllWithNotice(ctx, "Clean up temporary repositories", repo_module.LocalCopyPath())
	if err := initPushQueue(); err != nil {
		return err
	}
	return initBranchSyncQueue(graceful.GetManager().ShutdownContext())
}

// UpdateRepository updates a repository
func UpdateRepository(ctx context.Context, repo *repo_model.Repository, visibilityChanged bool) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = repo_module.UpdateRepository(ctx, repo, visibilityChanged); err != nil {
		return fmt.Errorf("updateRepository: %w", err)
	}

	return committer.Commit()
}

func UpdateRepositoryVisibility(ctx context.Context, repo *repo_model.Repository, isPrivate bool) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}

	defer committer.Close()

	repo.IsPrivate = isPrivate

	if err = repo_module.UpdateRepository(ctx, repo, true); err != nil {
		return fmt.Errorf("UpdateRepositoryVisibility: %w", err)
	}

	return committer.Commit()
}

func MakeRepoPublic(ctx context.Context, repo *repo_model.Repository) (err error) {
	return UpdateRepositoryVisibility(ctx, repo, false)
}

func MakeRepoPrivate(ctx context.Context, repo *repo_model.Repository) (err error) {
	return UpdateRepositoryVisibility(ctx, repo, true)
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
