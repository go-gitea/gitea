// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/sync"
	notify_service "code.gitea.io/gitea/services/notify"
)

// repoWorkingPool represents a working pool to order the parallel changes to the same repository
// TODO: use clustered lock (unique queue? or *abuse* cache)
var repoWorkingPool = sync.NewExclusivePool()

// TransferOwnership transfers all corresponding setting from old user to new one.
func TransferOwnership(ctx context.Context, doer, newOwner *user_model.User, repo *repo_model.Repository, teams []*organization.Team) error {
	if err := repo.LoadOwner(ctx); err != nil {
		return err
	}
	for _, team := range teams {
		if newOwner.ID != team.OrgID {
			return fmt.Errorf("team %d does not belong to organization", team.ID)
		}
	}

	oldOwner := repo.Owner

	repoWorkingPool.CheckIn(fmt.Sprint(repo.ID))
	if err := models.TransferOwnership(ctx, doer, newOwner.Name, repo); err != nil {
		repoWorkingPool.CheckOut(fmt.Sprint(repo.ID))
		return err
	}
	repoWorkingPool.CheckOut(fmt.Sprint(repo.ID))

	newRepo, err := repo_model.GetRepositoryByID(ctx, repo.ID)
	if err != nil {
		return err
	}

	for _, team := range teams {
		if err := models.AddRepository(ctx, team, newRepo); err != nil {
			return err
		}
	}

	notify_service.TransferRepository(ctx, doer, repo, oldOwner.Name)

	return nil
}

// ChangeRepositoryName changes all corresponding setting from old repository name to new one.
func ChangeRepositoryName(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, newRepoName string) error {
	log.Trace("ChangeRepositoryName: %s/%s -> %s", doer.Name, repo.Name, newRepoName)

	oldRepoName := repo.Name

	// Change repository directory name. We must lock the local copy of the
	// repo so that we can atomically rename the repo path and updates the
	// local copy's origin accordingly.

	repoWorkingPool.CheckIn(fmt.Sprint(repo.ID))
	if err := repo_model.ChangeRepositoryName(ctx, doer, repo, newRepoName); err != nil {
		repoWorkingPool.CheckOut(fmt.Sprint(repo.ID))
		return err
	}
	repoWorkingPool.CheckOut(fmt.Sprint(repo.ID))

	repo.Name = newRepoName
	notify_service.RenameRepository(ctx, doer, repo, oldRepoName)

	return nil
}

// StartRepositoryTransfer transfer a repo from one owner to a new one.
// it make repository into pending transfer state, if doer can not create repo for new owner.
func StartRepositoryTransfer(ctx context.Context, doer, newOwner *user_model.User, repo *repo_model.Repository, teams []*organization.Team) error {
	if err := models.TestRepositoryReadyForTransfer(repo.Status); err != nil {
		return err
	}

	// Admin is always allowed to transfer || user transfer repo back to his account
	if doer.IsAdmin || doer.ID == newOwner.ID {
		return TransferOwnership(ctx, doer, newOwner, repo, teams)
	}

	// If new owner is an org and user can create repos he can transfer directly too
	if newOwner.IsOrganization() {
		allowed, err := organization.CanCreateOrgRepo(ctx, newOwner.ID, doer.ID)
		if err != nil {
			return err
		}
		if allowed {
			return TransferOwnership(ctx, doer, newOwner, repo, teams)
		}
	}

	// In case the new owner would not have sufficient access to the repo, give access rights for read
	hasAccess, err := access_model.HasAccess(ctx, newOwner.ID, repo)
	if err != nil {
		return err
	}
	if !hasAccess {
		if err := repo_module.AddCollaborator(ctx, repo, newOwner); err != nil {
			return err
		}
		if err := repo_model.ChangeCollaborationAccessMode(ctx, repo, newOwner.ID, perm.AccessModeRead); err != nil {
			return err
		}
	}

	// Make repo as pending for transfer
	repo.Status = repo_model.RepositoryPendingTransfer
	if err := models.CreatePendingRepositoryTransfer(ctx, doer, newOwner, repo.ID, teams); err != nil {
		return err
	}

	// notify users who are able to accept / reject transfer
	notify_service.RepoPendingTransfer(ctx, doer, newOwner, repo)

	return nil
}
