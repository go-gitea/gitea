// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"

	"code.gitea.io/gitea/models"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
)

// DeleteDeployKey deletes deploy key from its repository authorized_keys file if needed.
func DeleteDeployKey(ctx context.Context, doer *user_model.User, id int64) error {
	dbCtx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := models.DeleteDeployKey(dbCtx, doer, id); err != nil {
		return err
	}
	if err := committer.Commit(); err != nil {
		return err
	}

	return asymkey_model.RewriteAllPublicKeys(ctx)
}
