// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"

	asymkey_model "gitea.dev/models/asymkey"
	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/services/audit"
)

// DeletePublicKey deletes SSH key information both in database and authorized_keys file.
func DeletePublicKey(ctx context.Context, doer *user_model.User, id int64) (err error) {
	key, err := asymkey_model.GetPublicKeyByID(ctx, id)
	if err != nil {
		return err
	}

	owner, err := user_model.GetUserByID(ctx, key.OwnerID)
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
		audit.Record(ctx, audit_model.UserKeyPrincipalRemove, doer, owner,
			fmt.Sprintf("Removed principal key %s of user %s.", key.Name, owner.Name), "key", key.Name)

		return RewriteAllPrincipalKeys(ctx)
	}

	audit.Record(ctx, audit_model.UserKeySSHRemove, doer, owner,
		fmt.Sprintf("Removed SSH key %s of user %s.", key.Fingerprint, owner.Name), "fingerprint", key.Fingerprint)

	return RewriteAllPublicKeys(ctx)
}
