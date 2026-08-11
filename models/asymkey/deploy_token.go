// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

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

// AddDeployToken adds a new token that authenticates git HTTP requests for one repository.
// The plaintext token is only readable on the returned key.
func AddDeployToken(ctx context.Context, repoID int64, name string, readOnly bool) (*DeployKey, error) {
	key := &DeployKey{
		RepoID: repoID,
		Type:   DeployKeyTypeToken,
		Name:   name,
		Token:  DeployTokenPrefix + util.CryptoRandomString(deployTokenLength),
		Mode:   util.Iif(readOnly, perm.AccessModeRead, perm.AccessModeWrite),
	}
	key.TokenHash = base.EncodeSha256(key.Token)

	return db.WithTx2(ctx, func(ctx context.Context) (*DeployKey, error) {
		if err := checkDeployKeyName(ctx, repoID, name); err != nil {
			return nil, err
		}
		return key, db.Insert(ctx, key)
	})
}

// VerifyDeployToken returns the deploy key which the given plaintext token authenticates.
func VerifyDeployToken(ctx context.Context, token string) (*DeployKey, error) {
	if !strings.HasPrefix(token, DeployTokenPrefix) { // spares a query for every password of a normal user
		return nil, ErrDeployKeyNotExist{}
	}

	key, exist, err := db.Get[DeployKey](ctx, builder.Eq{"token_hash": base.EncodeSha256(token)})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrDeployKeyNotExist{}
	}
	return key, nil
}
