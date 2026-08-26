// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package deploykey

import (
	"context"

	"gitea.dev/models/asymkey"
	"gitea.dev/models/db"

	"xorm.io/builder"
)

func (key *DeployKey) LoadPublicKey(ctx context.Context) (err error) {
	if key.PublicKey != nil {
		return nil
	}
	key.PublicKey, err = asymkey.GetPublicKeyByID(ctx, key.KeyID)
	return err
}

// GetDeployKeyByID returns deploy-key by given ID.
func GetDeployKeyByID(ctx context.Context, repoID, deployKeyID int64) (*DeployKey, error) {
	key, exist, err := db.Get[DeployKey](ctx, builder.Eq{"id": deployKeyID, "repo_id": repoID})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrDeployKeyNotExist{deployKeyID, 0, repoID}
	}
	return key, nil
}

// GetDeployKeyByRepoPublicKey returns deploy-key by given public key ID and repository ID.
func GetDeployKeyByRepoPublicKey(ctx context.Context, repoID, publicKeyID int64) (*DeployKey, error) {
	// the type is part of the condition because every token row carries key id 0
	key, exist, err := db.Get[DeployKey](ctx, builder.Eq{"key_id": publicKeyID, "repo_id": repoID, "key_type": KeyTypeSSH})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrDeployKeyNotExist{0, publicKeyID, repoID}
	}
	return key, nil
}

// IsDeployKeyExistByPublicKeyID return true if there is at least one deploy-key with the key id
func IsDeployKeyExistByPublicKeyID(ctx context.Context, keyID int64) (bool, error) {
	return db.Exist[DeployKey](ctx, builder.Eq{"key_id": keyID, "key_type": KeyTypeSSH})
}
