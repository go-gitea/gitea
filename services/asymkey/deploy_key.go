// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
)

// DeleteRepoDeployKeys deletes all deploy keys of a repository. permissions check should be done outside
func DeleteRepoDeployKeys(ctx context.Context, doer *user_model.User, repoID int64) (int, error) {
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

// checkDeployPerm Check if user has access to delete this key.
func checkDeployPerm(ctx context.Context, doer *user_model.User, repoID, keyID int64) error {
	if doer.IsAdmin {
		return nil
	}
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return fmt.Errorf("GetRepositoryByID: %w", err)
	}
	has, err := access_model.IsUserRepoAdmin(ctx, repo, doer)
	if err != nil {
		return fmt.Errorf("IsUserRepoAdmin: %w", err)
	} else if !has {
		return asymkey_model.ErrKeyAccessDenied{
			UserID: doer.ID,
			RepoID: repoID,
			KeyID:  keyID,
			Note:   "deploy",
		}
	}
	return nil
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
func DeleteDeployKey(ctx context.Context, doer *user_model.User, id int64) error {
	dbCtx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	key, err := asymkey_model.GetDeployKeyByID(ctx, id)
	if err != nil {
		if asymkey_model.IsErrDeployKeyNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetDeployKeyByID: %w", err)
	}

	if err := checkDeployPerm(ctx, doer, key.RepoID, key.ID); err != nil {
		return err
	}

	if err := deleteDeployKeyFromDB(dbCtx, key); err != nil {
		return err
	}
	if err := committer.Commit(); err != nil {
		return err
	}

	return RewriteAllPublicKeys(ctx)
}
