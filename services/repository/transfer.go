// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/sync"

	"github.com/unknwon/com"
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

	repoWorkingPool.CheckIn(com.ToStr(repo.ID))
	if err := models.TransferOwnership(doer, newOwner.Name, repo); err != nil {
		repoWorkingPool.CheckOut(com.ToStr(repo.ID))
		return err
	}
	repoWorkingPool.CheckOut(com.ToStr(repo.ID))

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

	repoWorkingPool.CheckIn(com.ToStr(repo.ID))
	if err := models.ChangeRepositoryName(doer, repo, newRepoName); err != nil {
		repoWorkingPool.CheckOut(com.ToStr(repo.ID))
		return err
	}
	repoWorkingPool.CheckOut(com.ToStr(repo.ID))

	notification.NotifyRenameRepository(doer, repo, oldRepoName)

	return nil
}

// StartRepositoryTransfer transfer a repo from one owner to a new one.
// it make repository into pending transfer state, if doer can not create repo for new owner.
func StartRepositoryTransfer(doer, newOwner *models.User, repo *models.Repository, teams []*models.Team) error {
	if repo.Status != models.RepositoryReady {
		return fmt.Errorf("repository is not ready for transfer")
	}

	// Admin is always allowed to transfer || user transfer repo back to his account
	if doer.IsAdmin || doer.ID == newOwner.ID {
		return TransferOwnership(doer, newOwner, repo, teams)
	}

	// If new owner is an org and user can create repos he can transfer directly too
	if newOwner.IsOrganization() {
		allowed, err := models.CanCreateOrgRepo(newOwner.ID, doer.ID)
		if err != nil {
			return err
		}
		if allowed {
			return TransferOwnership(doer, newOwner, repo, teams)
		}
	}

	// Block Transfer, new feature will come in v1.14.0
	// https://github.com/go-gitea/gitea/pull/14792
	return models.ErrCancelled{}
}
