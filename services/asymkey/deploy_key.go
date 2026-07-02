// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"

	asymkey_model "gitea.dev/models/asymkey"
	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/services/audit"
)

// DeleteRepoDeployKeys deletes all deploy keys of a repository. permissions check should be done outside
func DeleteRepoDeployKeys(ctx context.Context, repoID int64) (int, error) {
	deployKeys, err := db.Find[asymkey_model.DeployKey](ctx, asymkey_model.ListDeployKeysOptions{RepoID: repoID})
	if err != nil {
		return 0, fmt.Errorf("listDeployKeys: %w", err)
	}

	for _, dKey := range deployKeys {
		if err := deleteDeployKeyFromDB(ctx, dKey); err != nil {
			return 0, fmt.Errorf("deleteDeployKeys: %w", err)
		}
	}
	return len(deployKeys), nil
}

// deleteDeployKeyFromDB delete deploy keys from database
func deleteDeployKeyFromDB(ctx context.Context, key *asymkey_model.DeployKey) error {
	if _, err := db.DeleteByID[asymkey_model.DeployKey](ctx, key.ID); err != nil {
		return fmt.Errorf("delete deploy key [%d]: %w", key.ID, err)
	}

	// Check if this is the last reference to same key content.
	has, err := asymkey_model.IsDeployKeyExistByKeyID(ctx, key.KeyID)
	if err != nil {
		return err
	} else if !has {
		if _, err = db.DeleteByID[asymkey_model.PublicKey](ctx, key.KeyID); err != nil {
			return err
		}
	}

	return nil
}

// DeleteDeployKey deletes deploy key from its repository authorized_keys file if needed.
// Permissions check should be done outside.
func DeleteDeployKey(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, id int64) error {
	var deletedKey *asymkey_model.DeployKey
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		key, err := asymkey_model.GetDeployKeyByID(ctx, id)
		if err != nil {
			if asymkey_model.IsErrDeployKeyNotExist(err) {
				return nil
			}
			return fmt.Errorf("GetDeployKeyByID: %w", err)
		}

		if key.RepoID != repo.ID {
			return fmt.Errorf("deploy key %d does not belong to repository %d", id, repo.ID)
		}

		if err := deleteDeployKeyFromDB(ctx, key); err != nil {
			return err
		}
		deletedKey = key
		return nil
	}); err != nil {
		return err
	}

	if deletedKey != nil {
		audit.Record(ctx, audit_model.RepositoryDeployKeyRemove, doer, repo,
			fmt.Sprintf("Removed deploy key %s from repository %s.", deletedKey.Name, repo.FullName()),
			"deploy_key", deletedKey.Name)
	}

	return RewriteAllPublicKeys(ctx)
}
