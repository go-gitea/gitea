// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/globallock"
)

const ownerDeleteBatchSize = 100

// DeleteOwnerResources removes Codespace records owned by one user or organization scope.
func DeleteOwnerResources(ctx context.Context, ownerID int64) error {
	if ownerID <= 0 {
		return errors.New("owner_id must be positive")
	}
	return globallock.LockAndDo(ctx, codespaceOwnerRelationLockKey(ownerID), func(ctx context.Context) error {
		return deleteOwnerResourcesLocked(ctx, ownerID)
	})
}

// WithOwnerResourcesDeleted holds the owner relation lock after deleting Codespace resources.
func WithOwnerResourcesDeleted(ctx context.Context, ownerID int64, fn func(context.Context) error) error {
	if ownerID <= 0 {
		return errors.New("owner_id must be positive")
	}
	return globallock.LockAndDo(ctx, codespaceOwnerRelationLockKey(ownerID), func(ctx context.Context) error {
		if err := deleteOwnerResourcesLocked(ctx, ownerID); err != nil {
			return err
		}
		return fn(ctx)
	})
}

func deleteOwnerResourcesLocked(ctx context.Context, ownerID int64) error {
	if err := deleteOwnerManagersLocked(ctx, ownerID); err != nil {
		return err
	}
	if err := deleteOwnerRelatedCodespacesLocked(ctx, ownerID); err != nil {
		return err
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Where("owner_id = ?", ownerID).Delete(new(codespace_model.ManagerToken)); err != nil {
			return err
		}
		hasManager, err := db.GetEngine(ctx).Where("owner_id = ?", ownerID).Exist(new(codespace_model.Manager))
		if err != nil {
			return err
		}
		if hasManager {
			return fmt.Errorf("codespace managers still exist for owner %d", ownerID)
		}
		hasCodespace, err := ownerRelatedCodespaceExists(ctx, ownerID)
		if err != nil {
			return err
		}
		if hasCodespace {
			return fmt.Errorf("codespaces still exist for owner %d", ownerID)
		}
		return nil
	})
}

func deleteOwnerManagersLocked(ctx context.Context, ownerID int64) error {
	for {
		managerIDs, err := listOwnerManagerIDs(ctx, ownerID, ownerDeleteBatchSize)
		if err != nil {
			return err
		}
		if len(managerIDs) == 0 {
			return nil
		}
		for _, managerID := range managerIDs {
			if err := deleteOwnerManagerLocked(ctx, ownerID, managerID); err != nil {
				return err
			}
		}
	}
}

func listOwnerManagerIDs(ctx context.Context, ownerID int64, limit int) ([]int64, error) {
	var managers []*codespace_model.Manager
	if err := db.GetEngine(ctx).
		Where("owner_id = ?", ownerID).
		Asc("id").
		Limit(limit).
		Find(&managers); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(managers))
	for _, manager := range managers {
		ids = append(ids, manager.ID)
	}
	return ids, nil
}

func deleteOwnerManagerLocked(ctx context.Context, ownerID, managerID int64) error {
	return deleteManagerIdentityLocked(ctx, managerID, ownerDeleteBatchSize, func(manager *codespace_model.Manager) (bool, error) {
		if manager.OwnerID != ownerID {
			return false, nil
		}
		return true, nil
	})
}

func deleteOwnerRelatedCodespacesLocked(ctx context.Context, ownerID int64) error {
	for {
		codespaceUUIDs, err := listOwnerRelatedCodespaceUUIDs(ctx, ownerID, ownerDeleteBatchSize)
		if err != nil {
			return err
		}
		if len(codespaceUUIDs) == 0 {
			return nil
		}
		for _, codespaceUUID := range codespaceUUIDs {
			if err := deleteOwnerRelatedCodespace(ctx, ownerID, codespaceUUID); err != nil {
				return err
			}
		}
	}
}

func listOwnerRelatedCodespaceUUIDs(ctx context.Context, ownerID int64, limit int) ([]string, error) {
	var rows []*codespace_model.Codespace
	if err := db.GetEngine(ctx).
		Where("user_id = ? OR repo_id IN (SELECT id FROM repository WHERE owner_id = ?)", ownerID, ownerID).
		Asc("uuid").
		Limit(limit).
		Find(&rows); err != nil {
		return nil, err
	}
	uuids := make([]string, 0, len(rows))
	for _, row := range rows {
		uuids = append(uuids, row.UUID)
	}
	return uuids, nil
}

func deleteOwnerRelatedCodespace(ctx context.Context, ownerID int64, codespaceUUID string) error {
	return globallock.LockAndDo(ctx, codespaceLifecycleActionLockKey(codespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).ID(codespaceUUID).Get(codespace)
			if err != nil || !has {
				return err
			}
			related, err := codespaceBelongsToOwner(ctx, ownerID, codespace)
			if err != nil || !related {
				return err
			}
			return deleteCodespaceForFinal(ctx, codespaceUUID)
		})
	})
}

func codespaceBelongsToOwner(ctx context.Context, ownerID int64, codespace *codespace_model.Codespace) (bool, error) {
	if codespace.UserID == ownerID {
		return true, nil
	}
	if codespace.RepoID <= 0 {
		return false, nil
	}
	repo := new(repo_model.Repository)
	has, err := db.GetEngine(ctx).ID(codespace.RepoID).Get(repo)
	if err != nil || !has {
		return false, err
	}
	return repo.OwnerID == ownerID, nil
}

func ownerRelatedCodespaceExists(ctx context.Context, ownerID int64) (bool, error) {
	return db.GetEngine(ctx).
		Where("user_id = ? OR repo_id IN (SELECT id FROM repository WHERE owner_id = ?)", ownerID, ownerID).
		Exist(new(codespace_model.Codespace))
}
