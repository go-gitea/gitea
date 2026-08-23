// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
)

// CanUserDelete returns true if the user can delete the repository.
func CanUserDelete(ctx context.Context, repo *repo_model.Repository, user *user_model.User) (bool, error) {
	permission, err := access_model.GetIndividualUserRepoPermission(ctx, repo, user)
	if err != nil {
		return false, err
	}
	return permission.IsOwner(), nil
}
