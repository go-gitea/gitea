// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"

	asymkey_model "gitea.dev/models/asymkey"
	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	deploykey_model "gitea.dev/models/deploykey"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/services/audit"
)

// DeleteRepoDeployKeys deletes all deploy keys of a repository. permissions check should be done outside
func DeleteRepoDeployKeys(ctx context.Context, repoID int64) (int, error) {
	deployKeys, err := db.Find[deploykey_model.DeployKey](ctx, deploykey_model.ListDeployKeysOptions{RepoID: repoID})
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
func deleteDeployKeyFromDB(ctx context.Context, key *deploykey_model.DeployKey) error {
	if _, err := db.DeleteByID[deploykey_model.DeployKey](ctx, key.ID); err != nil {
		return fmt.Errorf("delete deploy key [%d]: %w", key.ID, err)
	}

	if key.KeyType == deploykey_model.KeyTypeToken { // a token has no public key to clean up
		return nil
	}

	// Check if this is the last reference to same key content.
	has, err := deploykey_model.IsDeployKeyExistByPublicKeyID(ctx, key.KeyID)
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
func DeleteDeployKey(ctx context.Context, repo *repo_model.Repository, id int64) (*deploykey_model.DeployKey, error) {
	deleted, err := db.WithTx2(ctx, func(ctx context.Context) (*deploykey_model.DeployKey, error) {
		key, err := deploykey_model.GetDeployKeyByID(ctx, repo.ID, id)
		if err != nil {
			return nil, err
		}
		return key, deleteDeployKeyFromDB(ctx, key)
	})
	if err != nil {
		return nil, err
	}

	audit.Record(ctx, audit_model.RepositoryDeployKeyRemove, repo, "deploy_key", deleted.Name)

	if deleted.KeyType == deploykey_model.KeyTypeToken {
		return deleted, nil // a token never appears in the authorized_keys file
	}

	return deleted, RewriteAllPublicKeys(ctx)
}
