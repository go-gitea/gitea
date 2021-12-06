// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package keys

import (
	"code.gitea.io/gitea/models/db"
	keys_model "code.gitea.io/gitea/models/keys"
	user_model "code.gitea.io/gitea/models/user"
)

// DeletePublicKey deletes SSH key information both in database and authorized_keys file.
func DeletePublicKey(doer *user_model.User, id int64) (err error) {
	key, err := keys_model.GetPublicKeyByID(id)
	if err != nil {
		return err
	}

	// Check if user has access to delete this key.
	if !doer.IsAdmin && doer.ID != key.OwnerID {
		return keys_model.ErrKeyAccessDenied{
			UserID: doer.ID,
			KeyID:  key.ID,
			Note:   "public",
		}
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = keys_model.DeletePublicKeys(ctx, id); err != nil {
		return err
	}

	if err = committer.Commit(); err != nil {
		return err
	}
	committer.Close()

	if key.Type == keys_model.KeyTypePrincipal {
		return keys_model.RewriteAllPrincipalKeys()
	}

	return keys_model.RewriteAllPublicKeys()
}
