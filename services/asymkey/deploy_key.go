// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"code.gitea.io/gitea/models"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/db"
)

// DeleteDeployKey deletes deploy key from its repository authorized_keys file if needed.
func DeleteDeployKey(doer *user_model.User, id int64) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := models.DeleteDeployKey(ctx, doer, id); err != nil {
		return err
	}
	if err := committer.Commit(); err != nil {
		return err
	}

	return asymkey_model.RewriteAllPublicKeys()
}
