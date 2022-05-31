// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models"
	admin_model "code.gitea.io/gitea/models/admin"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	pull_service "code.gitea.io/gitea/services/pull"
)

// CreateRepository creates a repository for the user/organization.
func CreateRepository(doer, owner *user_model.User, opts models.CreateRepoOptions) (*repo_model.Repository, error) {
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

	repo, err := CreateRepository(authUser, owner, models.CreateRepoOptions{
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
	admin_model.RemoveAllWithNotice(db.DefaultContext, "Clean up temporary repository uploads", setting.Repository.Upload.TempPath)
	admin_model.RemoveAllWithNotice(db.DefaultContext, "Clean up temporary repositories", repo_module.LocalCopyPath())
	return initPushQueue()
}
