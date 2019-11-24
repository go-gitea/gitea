// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
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
	if err := models.DeleteRepository(doer, repo.OwnerID, repo.ID); err != nil {
		return err
	}

	notification.NotifyDeleteRepository(doer, repo)

	return nil
}
