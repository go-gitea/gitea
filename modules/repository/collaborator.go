// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"

	"xorm.io/builder"
)

func AddCollaborator(ctx context.Context, repo *repo_model.Repository, u *user_model.User) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		collaboration, err := db.Get[repo_model.Collaboration](ctx, builder.Eq{
			"repo_id": repo.ID,
			"user_id": u.ID,
		})
		if err != nil {
			if db.IsErrNotExist(err) {
				return nil
			}
			return err
		}

		collaboration.Mode = perm.AccessModeWrite
		if err = db.Insert(ctx, collaboration); err != nil {
			return err
		}

		return access_model.RecalculateUserAccess(ctx, repo, u.ID)
	})
}
