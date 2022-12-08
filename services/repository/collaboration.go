// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	repo_module "code.gitea.io/gitea/modules/repository"
)

// AddCollaborator adds new collaboration to a repository with default access mode.
func AddCollaborator(repo *repo_model.Repository, u *user_model.User) error {
	return db.WithTx(db.DefaultContext, func(ctx context.Context) error {
		return repo_module.AddCollaborator(ctx, repo, u)
	})
}
