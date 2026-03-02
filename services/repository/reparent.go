// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
	notify_service "code.gitea.io/gitea/services/notify"
)

// StartRepositoryReparent marks the repository as pending reparenting
func StartRepositoryReparent(ctx context.Context, doer *user_model.User, source *repo_model.Repository, targetOwnerID int64) (*repo_model.Repository, error) {
	var targetOwner *user_model.User
	err := db.WithTx(ctx, func(ctx context.Context) error {
		var err error
		targetOwner, err = user_model.GetUserByID(ctx, targetOwnerID)
		if err != nil {
			return err
		}

		if err := repo_model.TestRepositoryReadyForTransfer(source.Status); err != nil {
			return err
		}

		exist, err := repo_model.IsRepositoryTransferExist(ctx, source.ID)
		if err != nil {
			return err
		}
		if exist {
			return repo_model.ErrRepoTransferInProgress{
				Uname: source.OwnerName,
				Name:  source.Name,
			}
		}

		source.Status = repo_model.RepositoryPendingReparent
		if err := repo_model.UpdateRepositoryColsNoAutoTime(ctx, source, "status"); err != nil {
			return err
		}

		transfer := &repo_model.RepoTransfer{
			RepoID:      source.ID,
			RecipientID: source.OwnerID, // Sentinel for reparenting
			CreatedUnix: timeutil.TimeStampNow(),
			UpdatedUnix: timeutil.TimeStampNow(),
			DoerID:      doer.ID,
			TeamIDs:     []int64{targetOwnerID},
		}

		return db.Insert(ctx, transfer)
	})
	if err == nil {
		notify_service.RepoPendingTransfer(ctx, doer, targetOwner, source)
	}
	return source, err
}

// AcceptReparent accepts a pending reparenting request
func AcceptReparent(ctx context.Context, doer *user_model.User, source *repo_model.Repository) error {
	oldOwnerName := source.OwnerName
	err := db.WithTx(ctx, func(ctx context.Context) error {
		repoTransfer, err := repo_model.GetPendingRepositoryTransfer(ctx, source)
		if err != nil {
			return err
		}

		if err := repoTransfer.LoadAttributes(ctx); err != nil {
			return err
		}

		if !repoTransfer.IsReparent(ctx) {
			return errors.New("pending operation is not a reparent request")
		}

		targetOwnerID := repoTransfer.GetTargetOwnerID()
		if targetOwnerID == 0 {
			return errors.New("invalid target owner ID")
		}

		targetOwner, err := user_model.GetUserByID(ctx, targetOwnerID)
		if err != nil {
			return err
		}

		targetRepo, err := repo_model.GetUserFork(ctx, source.ID, targetOwnerID)
		if err != nil {
			return err
		}

		if targetRepo == nil {
			// Check if a repository with the same name already exists
			exists, err := repo_model.IsRepositoryModelExist(ctx, targetOwner, source.Name)
			if err != nil {
				return err
			}
			if exists {
				return repo_model.ErrRepoAlreadyExist{
					Uname: targetOwner.Name,
					Name:  source.Name,
				}
			}

			// Create the fork
			targetRepo, err = ForkRepository(ctx, doer, targetOwner, ForkRepoOptions{
				BaseRepo:    source,
				Name:        source.Name,
				Description: source.Description,
			})
			if err != nil {
				return err
			}
		}

		// Swap parent and fork
		if err := repo_model.ReparentFork(ctx, targetRepo.ID, source.ID); err != nil {
			return err
		}

		source.Status = repo_model.RepositoryReady
		if err := repo_model.UpdateRepositoryColsNoAutoTime(ctx, source, "status"); err != nil {
			return err
		}

		return repo_model.DeleteRepositoryTransfer(ctx, source.ID)
	})
	if err == nil {
		notify_service.TransferRepository(ctx, doer, source, oldOwnerName)
	}
	return err
}

// CancelRepositoryReparent cancels a pending reparenting request
func CancelRepositoryReparent(ctx context.Context, doer *user_model.User, source *repo_model.Repository) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		repoTransfer, err := repo_model.GetPendingRepositoryTransfer(ctx, source)
		if err != nil {
			if repo_model.IsErrNoPendingTransfer(err) {
				return nil
			}
			return err
		}

		if err := repoTransfer.LoadAttributes(ctx); err != nil {
			return err
		}

		if !repoTransfer.IsReparent(ctx) {
			return errors.New("pending operation is not a reparent request")
		}

		source.Status = repo_model.RepositoryReady
		if err := repo_model.UpdateRepositoryColsNoAutoTime(ctx, source, "status"); err != nil {
			return err
		}

		return repo_model.DeleteRepositoryTransfer(ctx, source.ID)
	})
}
