// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/sync"
)

// repoWorkingPool represents a working pool to order the parallel changes to the same repository
var repoWorkingPool = sync.NewExclusivePool()

// TransferOwnership transfers all corresponding setting from old user to new one.
func TransferOwnership(doer, newOwner *models.User, repo *models.Repository, teams []*models.Team) error {
	if err := repo.GetOwner(); err != nil {
		return err
	}
	for _, team := range teams {
		if newOwner.ID != team.OrgID {
			return fmt.Errorf("team %d does not belong to organization", team.ID)
		}
	}

	oldOwner := repo.Owner

	repoWorkingPool.CheckIn(fmt.Sprint(repo.ID))
	if err := models.TransferOwnership(doer, newOwner.Name, repo); err != nil {
		repoWorkingPool.CheckOut(fmt.Sprint(repo.ID))
		return err
	}
	repoWorkingPool.CheckOut(fmt.Sprint(repo.ID))

	newRepo, err := models.GetRepositoryByID(repo.ID)
	if err != nil {
		return err
	}

	for _, team := range teams {
		if err := team.AddRepository(newRepo); err != nil {
			return err
		}
	}

	notification.NotifyTransferRepository(doer, repo, oldOwner.Name)

	return nil
}

// ChangeRepositoryName changes all corresponding setting from old repository name to new one.
func ChangeRepositoryName(doer *models.User, repo *models.Repository, newRepoName string) error {
	oldRepoName := repo.Name

	// Change repository directory name. We must lock the local copy of the
	// repo so that we can atomically rename the repo path and updates the
	// local copy's origin accordingly.

	repoWorkingPool.CheckIn(fmt.Sprint(repo.ID))
	if err := models.ChangeRepositoryName(doer, repo, newRepoName); err != nil {
		repoWorkingPool.CheckOut(fmt.Sprint(repo.ID))
		return err
	}
	repoWorkingPool.CheckOut(fmt.Sprint(repo.ID))

	notification.NotifyRenameRepository(doer, repo, oldRepoName)

	return nil
}

// StartRepositoryTransfer transfer a repo from one owner to a new one.
// it marks the repository transfer as "pending",
// if the new owner is a user or if he dont have access to the new place.
func StartRepositoryTransfer(doer, newOwner *models.User, repo *models.Repository, teams []*models.Team) error {
	if err := models.TestRepositoryReadyForTransfer(repo.Status); err != nil {
		return err
	}

	// if user is allowed to directly transfer, do it
	if doer.IsAdmin || newOwner.IsOrganization() && newOwner.HasMemberWithUserID(doer.ID) {
		return TransferOwnership(doer, newOwner, repo, teams)
	}

	// Make repo as pending for transfer
	repo.Status = models.RepositoryPendingTransfer
	return models.StartRepositoryTransfer(doer, newOwner, repo.ID, teams)
}
