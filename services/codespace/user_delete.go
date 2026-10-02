// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/modules/globallock"
)

const userDeleteBatchSize = 100

// DeleteUserResources removes Codespace resources owned by one user.
func DeleteUserResources(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return errors.New("user_id must be positive")
	}
	return globallock.LockAndDo(ctx, codespaceUserRelationLockKey(userID), func(ctx context.Context) error {
		return deleteUserResourcesLocked(ctx, userID)
	})
}

// WithUserResourcesDeleted holds the user relation lock after deleting Codespace resources.
func WithUserResourcesDeleted(ctx context.Context, userID int64, fn func(context.Context) error) error {
	if userID <= 0 {
		return errors.New("user_id must be positive")
	}
	return globallock.LockAndDo(ctx, codespaceUserRelationLockKey(userID), func(ctx context.Context) error {
		if err := deleteUserResourcesLocked(ctx, userID); err != nil {
			return err
		}
		return fn(ctx)
	})
}

func deleteUserResourcesLocked(ctx context.Context, userID int64) error {
	if err := deleteUserManagersLocked(ctx, userID); err != nil {
		return err
	}
	if err := deleteUserCodespacesLocked(ctx, userID); err != nil {
		return err
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		var secrets []*codespace_model.UserSecret
		if err := db.GetEngine(ctx).Where("user_id = ?", userID).Find(&secrets); err != nil {
			return err
		}
		for _, secret := range secrets {
			if _, err := db.GetEngine(ctx).Where("secret_id = ?", secret.ID).Delete(new(codespace_model.UserSecretRepository)); err != nil {
				return err
			}
		}
		if _, err := db.GetEngine(ctx).Where("user_id = ?", userID).Delete(new(codespace_model.UserSecret)); err != nil {
			return err
		}
		var authorizations []*codespace_model.PermissionAuthorization
		if err := db.GetEngine(ctx).Where("user_id = ?", userID).Find(&authorizations); err != nil {
			return err
		}
		for _, authorization := range authorizations {
			if _, err := db.GetEngine(ctx).Where("authorization_id = ?", authorization.ID).Delete(new(codespace_model.PermissionRepository)); err != nil {
				return err
			}
		}
		if _, err := db.GetEngine(ctx).Where("user_id = ?", userID).Delete(new(codespace_model.PermissionAuthorization)); err != nil {
			return err
		}
		hasManager, err := db.GetEngine(ctx).Where("user_id = ?", userID).Exist(new(codespace_model.Manager))
		if err != nil {
			return err
		}
		if hasManager {
			return fmt.Errorf("codespace managers still exist for user %d", userID)
		}
		hasCodespace, err := db.GetEngine(ctx).Where("user_id = ?", userID).Exist(new(codespace_model.Codespace))
		if err != nil {
			return err
		}
		if hasCodespace {
			return fmt.Errorf("codespaces still exist for user %d", userID)
		}
		return nil
	})
}

func deleteUserManagersLocked(ctx context.Context, userID int64) error {
	for {
		var managers []*codespace_model.Manager
		if err := db.GetEngine(ctx).
			Where("user_id = ?", userID).
			Asc("id").
			Limit(userDeleteBatchSize).
			Find(&managers); err != nil {
			return err
		}
		if len(managers) == 0 {
			return nil
		}
		for _, manager := range managers {
			err := deleteManagerIdentityLocked(ctx, manager.ID, userDeleteBatchSize, func(current *codespace_model.Manager) (bool, error) {
				return current.UserID == userID, nil
			})
			if err != nil {
				return err
			}
		}
	}
}

func deleteUserCodespacesLocked(ctx context.Context, userID int64) error {
	for {
		var rows []*codespace_model.Codespace
		if err := db.GetEngine(ctx).
			Where("user_id = ?", userID).
			Asc("id").
			Limit(userDeleteBatchSize).
			Find(&rows); err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			if err := deleteUserCodespace(ctx, userID, row.UUID); err != nil {
				return err
			}
		}
	}
}

func deleteUserCodespace(ctx context.Context, userID int64, codespaceUUID string) error {
	return globallock.LockAndDo(ctx, codespaceStateLockKey(codespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).Where("uuid = ?", codespaceUUID).Get(codespace)
			if err != nil || !has || codespace.UserID != userID {
				return err
			}
			return deleteCodespaceForFinal(ctx, codespaceUUID)
		})
	})
}
