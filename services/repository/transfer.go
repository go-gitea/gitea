// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification"
)

// TransferOwnership transfers all corresponding setting from old user to new one.
func TransferOwnership(doer *models.User, newOwnerName string, repo *models.Repository) error {
	if err := repo.GetOwner(); err != nil {
		return err
	}

	oldOwner := repo.Owner

	if err := models.TransferOwnership(doer, newOwnerName, repo); err != nil {
		return err
	}

	if err := models.NewRepoRedirect(oldOwner.ID, repo.ID, repo.Name, repo.Name); err != nil {
		return fmt.Errorf("NewRepoRedirect: %v", err)
	}

	notification.NotifyTransferRepository(doer, repo, oldOwner.Name)

	return nil
}

// ChangeRepositoryName changes all corresponding setting from old repository name to new one.
func ChangeRepositoryName(doer *models.User, repo *models.Repository, newRepoName string) error {
	oldRepoName := repo.Name

	if err := models.ChangeRepositoryName(doer, repo, newRepoName); err != nil {
		return err
	}

	if err := repo.GetOwner(); err != nil {
		return err
	}

	if err := models.NewRepoRedirect(repo.Owner.ID, repo.ID, oldRepoName, newRepoName); err != nil {
		return err
	}

	notification.NotifyRenameRepository(doer, repo, oldRepoName)

	return nil
}
