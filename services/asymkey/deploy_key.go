// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
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
func DeleteDeployKey(ctx context.Context, doer *user_model.User, id int64) error {
	dbCtx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	key, err := asymkey_model.GetDeployKeyByID(dbCtx, id)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return fmt.Errorf("GetDeployKeyByID: %w", err)
	}

	repo, err := repo_model.GetRepositoryByID(dbCtx, key.RepoID)
	if err != nil {
		return fmt.Errorf("GetRepositoryByID: %w", err)
	}

	if err := models.DeleteDeployKey(dbCtx, doer, id); err != nil {
		return err
	}
	if err := committer.Commit(); err != nil {
		return err
	}

	audit.RecordRepositoryDeployKeyRemove(ctx, doer, repo, key)

	return RewriteAllPublicKeys(ctx)
}
