// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"

	asymkey_model "gitea.dev/models/asymkey"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
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

	if key.Type != asymkey_model.DeployKeyTypeSSH { // a token has no public key to clean up
		return nil
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

// DeleteDeployKey deletes deploy key from its repository authorized_keys file if needed,
// and returns the key it deleted. Permissions check should be done outside.
func DeleteDeployKey(ctx context.Context, repo *repo_model.Repository, id int64) (*asymkey_model.DeployKey, error) {
	deleted, err := db.WithTx2(ctx, func(ctx context.Context) (*asymkey_model.DeployKey, error) {
		key, err := asymkey_model.GetDeployKeyByID(ctx, id)
		if err != nil {
			return nil, err
		}

		if key.RepoID != repo.ID {
			return nil, fmt.Errorf("deploy key %d does not belong to repository %d", id, repo.ID)
		}

		return key, deleteDeployKeyFromDB(ctx, key)
	})
	if err != nil {
		return nil, err
	}
	if deleted.Type != asymkey_model.DeployKeyTypeSSH {
		return deleted, nil // a token never appears in the authorized_keys file
	}

	return deleted, RewriteAllPublicKeys(ctx)
}
