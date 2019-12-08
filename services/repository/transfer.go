// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification"

	"github.com/unknwon/com"
)

// TransferOwnership transfers all corresponding setting from old user to new one.
func TransferOwnership(doer *models.User, newOwnerName string, repo *models.Repository) error {
	if err := repo.GetOwner(); err != nil {
		return err
	}

	oldOwner := repo.Owner

	models.RepoWorkingPool.CheckIn(com.ToStr(repo.ID))
	if err := models.TransferOwnership(doer, newOwnerName, repo); err != nil {
		models.RepoWorkingPool.CheckOut(com.ToStr(repo.ID))
		return err
	}
	models.RepoWorkingPool.CheckOut(com.ToStr(repo.ID))

	notification.NotifyTransferRepository(doer, repo, oldOwner.Name)

	return nil
}

// ChangeRepositoryName changes all corresponding setting from old repository name to new one.
func ChangeRepositoryName(doer *models.User, repo *models.Repository, newRepoName string) error {
	oldRepoName := repo.Name

	// Change repository directory name. We must lock the local copy of the
	// repo so that we can atomically rename the repo path and updates the
	// local copy's origin accordingly.

	models.RepoWorkingPool.CheckIn(com.ToStr(repo.ID))
	if err := models.ChangeRepositoryName(doer, repo, newRepoName); err != nil {
		models.RepoWorkingPool.CheckOut(com.ToStr(repo.ID))
		return err
	}
	models.RepoWorkingPool.CheckOut(com.ToStr(repo.ID))

	notification.NotifyRenameRepository(doer, repo, oldRepoName)

	return nil
}
