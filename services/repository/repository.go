// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	pull_service "code.gitea.io/gitea/services/pull"
)

// CreateRepository creates a repository for the user/organization.
func CreateRepository(doer, owner *models.User, opts models.CreateRepoOptions) (*models.Repository, error) {
	repo, err := models.CreateRepository(doer, owner, opts)
	if err != nil {
		if repo != nil {
			if errDelete := models.DeleteRepository(doer, owner.ID, repo.ID); errDelete != nil {
				log.Error("Rollback deleteRepository: %v", errDelete)
			}
		}
		return nil, err
	}

	notification.NotifyCreateRepository(doer, owner, repo)

	return repo, nil
}

// ForkRepository forks a repository
func ForkRepository(doer, u *models.User, oldRepo *models.Repository, name, desc string) (*models.Repository, error) {
	repo, err := models.ForkRepository(doer, u, oldRepo, name, desc)
	if err != nil {
		if repo != nil {
			if errDelete := models.DeleteRepository(doer, u.ID, repo.ID); errDelete != nil {
				log.Error("Rollback deleteRepository: %v", errDelete)
			}
		}
		return nil, err
	}

	notification.NotifyForkRepository(doer, oldRepo, repo)

	return repo, nil
}

// DeleteRepository deletes a repository for a user or organization.
func DeleteRepository(doer *models.User, repo *models.Repository) error {
	if err := pull_service.CloseRepoBranchesPulls(doer, repo); err != nil {
		log.Error("CloseRepoBranchesPulls failed: %v", err)
	}

	if err := models.DeleteRepository(doer, repo.OwnerID, repo.ID); err != nil {
		return err
	}

	notification.NotifyDeleteRepository(doer, repo)

	return nil
}

// PushCreateRepo creates a repository when a new repository is pushed to an appropriate namespace
func PushCreateRepo(authUser, owner *models.User, repoName string) (*models.Repository, error) {
	if !authUser.IsAdmin {
		if owner.IsOrganization() {
			if ok, err := owner.CanCreateOrgRepo(authUser.ID); err != nil {
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
		IsPrivate: true,
	})
	if err != nil {
		return nil, err
	}

	return repo, nil
}
