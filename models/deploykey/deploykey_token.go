// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package deploykey

import (
	"context"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/modules/base"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

const (
	DeployTokenPrefix = "gdt_" // lets a secret scanner recognize a leaked token
	deployTokenLength = 43     // 256 bits of entropy over the 62 alphanumerical characters
)

func (key *DeployKey) generateToken() {
	key.Token = DeployTokenPrefix + util.CryptoRandomString(deployTokenLength)
	key.TokenHash = base.EncodeSha256(key.Token)
	key.Fingerprint = key.Token[:len(DeployTokenPrefix)+2] + "********" + key.Token[len(key.Token)-2:]
}

// AddDeployKeyToken adds a token that authenticates git HTTP requests for one repository.
// The plaintext token is only readable on the returned key.
func AddDeployKeyToken(ctx context.Context, repoID int64, name string, readOnly bool) (*DeployKey, error) {
	key := &DeployKey{
		RepoID:  repoID,
		KeyType: KeyTypeToken,
		Name:    name,
		Mode:    util.Iif(readOnly, perm.AccessModeRead, perm.AccessModeWrite),
	}
	key.generateToken()

	return db.WithTx2(ctx, func(ctx context.Context) (*DeployKey, error) {
		if err := checkDeployKeyName(ctx, repoID, name); err != nil {
			return nil, err
		}
		return key, db.Insert(ctx, key)
	})
}

// RegenerateDeployKeyToken replaces the token value of an existing deploy token, keeping its name and access mode.
func RegenerateDeployKeyToken(ctx context.Context, repoID, keyID int64) (*DeployKey, error) {
	key, err := GetDeployKeyByID(ctx, repoID, keyID)
	if err != nil {
		return nil, err
	}
	if key.KeyType != KeyTypeToken {
		return nil, ErrDeployKeyNotExist{keyID, 0, repoID}
	}

	key.generateToken()
	_, err = db.GetEngine(ctx).ID(key.ID).Cols("token_hash").NoAutoTime().Update(key)
	return key, err
}

// VerifyDeployKeyToken returns the deploy-key which the given plaintext token authenticates.
func VerifyDeployKeyToken(ctx context.Context, token string) (*DeployKey, error) {
	if !strings.HasPrefix(token, DeployTokenPrefix) { // spares a query for every password of a normal user
		return nil, ErrDeployKeyNotExist{}
	}

	key, exist, err := db.Get[DeployKey](ctx, builder.Eq{"token_hash": base.EncodeSha256(token), "key_type": KeyTypeToken})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrDeployKeyNotExist{}
	}
	return key, nil
}
