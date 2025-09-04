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
	key := &asymkey_model.PublicKey{
		OwnerID:       ownerID,
		Name:          content,
		Content:       content,
		Mode:          perm.AccessModeWrite,
		Type:          asymkey_model.KeyTypePrincipal,
		LoginSourceID: authSourceID,
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		// Principals cannot be duplicated.
		has, err := db.GetEngine(ctx).
			Where("content = ? AND type = ?", content, asymkey_model.KeyTypePrincipal).
			Get(new(asymkey_model.PublicKey))
		if err != nil {
			return err
		} else if has {
			return asymkey_model.ErrKeyAlreadyExist{
				Content: content,
			}
		}

		if err = db.Insert(ctx, key); err != nil {
			return fmt.Errorf("addKey: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return key, RewriteAllPrincipalKeys(ctx)
}
