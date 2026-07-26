// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
)

// CanUserDelete returns true if user could delete the repository.
// Same criteria as IsUserRepoAdmin (site admin, owner, repo admin access).
func CanUserDelete(ctx context.Context, repo *repo_model.Repository, user *user_model.User) (bool, error) {
	return access_model.IsUserRepoAdmin(ctx, repo, user)
}
