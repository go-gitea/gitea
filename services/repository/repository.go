// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	system_model "code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	pull_service "code.gitea.io/gitea/services/pull"
)

// CreateRepository creates a repository for the user/organization.
func CreateRepository(doer, owner *user_model.User, opts repo_module.CreateRepoOptions) (*repo_model.Repository, error) {
	repo, err := repo_module.CreateRepository(doer, owner, opts)
	if err != nil {
		// No need to rollback here we should do this in CreateRepository...
		return nil, err
	}

	notification.NotifyCreateRepository(doer, owner, repo)

	return repo, nil
}

// DeleteRepository deletes a repository for a user or organization.
func DeleteRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, notify bool) error {
	if err := pull_service.CloseRepoBranchesPulls(ctx, doer, repo); err != nil {
		log.Error("CloseRepoBranchesPulls failed: %v", err)
	}

	if notify {
		// If the repo itself has webhooks, we need to trigger them before deleting it...
		notification.NotifyDeleteRepository(doer, repo)
	}

	if err := models.DeleteRepository(doer, repo.OwnerID, repo.ID); err != nil {
		return err
	}

	return packages_model.UnlinkRepositoryFromAllPackages(ctx, repo.ID)
}

// PushCreateRepo creates a repository when a new repository is pushed to an appropriate namespace
func PushCreateRepo(authUser, owner *user_model.User, repoName string) (*repo_model.Repository, error) {
	if !authUser.IsAdmin {
		if owner.IsOrganization() {
			if ok, err := organization.CanCreateOrgRepo(owner.ID, authUser.ID); err != nil {
				return nil, err
			} else if !ok {
				return nil, fmt.Errorf("cannot push-create repository for org")
			}
		} else if authUser.ID != owner.ID {
			return nil, fmt.Errorf("cannot push-create repository for another user")
		}
	}

	repo, err := CreateRepository(authUser, owner, repo_module.CreateRepoOptions{
		Name:      repoName,
		IsPrivate: setting.Repository.DefaultPushCreatePrivate,
	})
	if err != nil {
		return nil, err
	}

	return repo, nil
}

// Init start repository service
func Init() error {
	repo_module.LoadRepoConfig()
	system_model.RemoveAllWithNotice(db.DefaultContext, "Clean up temporary repository uploads", setting.Repository.Upload.TempPath)
	system_model.RemoveAllWithNotice(db.DefaultContext, "Clean up temporary repositories", repo_module.LocalCopyPath())
	return initPushQueue()
}

// UpdateRepository updates a repository
func UpdateRepository(repo *repo_model.Repository, visibilityChanged bool) (err error) {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = repo_module.UpdateRepository(ctx, repo, visibilityChanged); err != nil {
		return fmt.Errorf("updateRepository: %w", err)
	}

	return committer.Commit()
}

// LinkedRepository returns the linked repo if any
func LinkedRepository(a *repo_model.Attachment) (*repo_model.Repository, unit.Type, error) {
	if a.IssueID != 0 {
		iss, err := issues_model.GetIssueByID(db.DefaultContext, a.IssueID)
		if err != nil {
			return nil, unit.TypeIssues, err
		}
		repo, err := repo_model.GetRepositoryByID(iss.RepoID)
		unitType := unit.TypeIssues
		if iss.IsPull {
			unitType = unit.TypePullRequests
		}
		return repo, unitType, err
	} else if a.ReleaseID != 0 {
		rel, err := repo_model.GetReleaseByID(db.DefaultContext, a.ReleaseID)
		if err != nil {
			return nil, unit.TypeReleases, err
		}
		repo, err := repo_model.GetRepositoryByID(rel.RepoID)
		return repo, unit.TypeReleases, err
	}
	return nil, -1, nil
}
