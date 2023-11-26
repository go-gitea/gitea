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
)

func AddCollaborator(ctx context.Context, repo *repo_model.Repository, u *user_model.User) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		collaboration := &repo_model.Collaboration{}

		has, err := db.GetEngine(ctx).Where("repo_id=? AND user_id=?", repo.ID, u.ID).
			NoAutoCondition().
			Get(ctx, collaboration)
		if err != nil {
			return err
		} else if has {
			return nil
		}
		collaboration.Mode = perm.AccessModeWrite

		if err = db.Insert(ctx, collaboration); err != nil {
			return err
		}

		return access_model.RecalculateUserAccess(ctx, repo, u.ID)
	})
}
