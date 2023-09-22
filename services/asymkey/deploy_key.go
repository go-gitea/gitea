// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"errors"
	"fmt"

	"code.gitea.io/gitea/models"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/audit"
)

// DeleteDeployKey deletes deploy key from its repository authorized_keys file if needed.
func DeleteDeployKey(doer *user_model.User, id int64) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	key, err := asymkey_model.GetDeployKeyByID(ctx, id)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return fmt.Errorf("GetDeployKeyByID: %w", err)
	}

	repo, err := repo_model.GetRepositoryByID(ctx, key.RepoID)
	if err != nil {
		return fmt.Errorf("GetRepositoryByID: %w", err)
	}

	if err := models.DeleteDeployKey(ctx, doer, id); err != nil {
		return err
	}
	if err := committer.Commit(); err != nil {
		return err
	}

	audit.Record(audit.RepositoryDeployKeyRemove, doer, repo, key, "Removed deploy key %s.", key.Name)

	return asymkey_model.RewriteAllPublicKeys()
}
