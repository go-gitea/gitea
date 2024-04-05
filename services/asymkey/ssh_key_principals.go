// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
)

// AddPrincipalKey adds new principal to database and authorized_principals file.
func AddPrincipalKey(ctx context.Context, ownerID int64, content string, authSourceID int64) (*asymkey_model.PublicKey, error) {
	dbCtx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	// Principals cannot be duplicated.
	has, err := db.GetEngine(dbCtx).
		Where("content = ? AND type = ?", content, asymkey_model.KeyTypePrincipal).
		Get(new(asymkey_model.PublicKey))
	if err != nil {
		return nil, err
	} else if has {
		return nil, asymkey_model.ErrKeyAlreadyExist{
			Content: content,
		}
	}

	key := &asymkey_model.PublicKey{
		OwnerID:       ownerID,
		Name:          content,
		Content:       content,
		Mode:          perm.AccessModeWrite,
		Type:          asymkey_model.KeyTypePrincipal,
		LoginSourceID: authSourceID,
	}
	if err = db.Insert(dbCtx, key); err != nil {
		return nil, fmt.Errorf("addKey: %w", err)
	}

	if err = committer.Commit(); err != nil {
		return nil, err
	}

	committer.Close()

	return key, RewriteAllPrincipalKeys(ctx)
}
