// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
)

// DeletePublicKey deletes SSH key information both in database and authorized_keys file.
func DeletePublicKey(ctx context.Context, doer *user_model.User, id int64) (err error) {
	key, err := asymkey_model.GetPublicKeyByID(ctx, id)
	if err != nil {
		return err
	}

	// Check if user has access to delete this key.
	if !doer.IsAdmin && doer.ID != key.OwnerID {
		return asymkey_model.ErrKeyAccessDenied{
			UserID: doer.ID,
			KeyID:  key.ID,
			Note:   "public",
		}
	}

	if _, err = db.DeleteByID[asymkey_model.PublicKey](ctx, id); err != nil {
		return err
	}

	if key.Type == asymkey_model.KeyTypePrincipal {
		return RewriteAllPrincipalKeys(ctx)
	}

	return RewriteAllPublicKeys(ctx)
}
