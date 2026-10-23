// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package deploykey

import (
	"context"

	"gitea.dev/models/asymkey"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// AddDeployKeySSH add new deploy-key to database and authorized_keys file.
func AddDeployKeySSH(ctx context.Context, repoID int64, name, content string, accessMode perm.AccessMode) (*DeployKey, error) {
	if accessMode != perm.AccessModeRead && accessMode != perm.AccessModeWrite {
		return nil, util.NewInvalidArgumentErrorf("invalid access mode")
	}
	return db.WithTx2(ctx, func(ctx context.Context) (*DeployKey, error) {
		pkey, err := asymkey.FindOrAddDeployPublicKey(ctx, content)
		if err != nil {
			return nil, err
		}
		if has, err := db.Exist[DeployKey](ctx, builder.Eq{"repo_id": repoID, "key_id": pkey.ID}); err != nil {
			return nil, err
		} else if has {
			return nil, ErrDeployKeyAlreadyExist{pkey.ID, repoID}
		}
		if err := checkDeployKeyName(ctx, repoID, name); err != nil {
			return nil, err
		}

		key := &DeployKey{KeyID: pkey.ID, RepoID: repoID, KeyType: KeyTypeSSH, Name: name, Fingerprint: pkey.Fingerprint, Mode: accessMode}
		return key, db.Insert(ctx, key)
	})
}

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
