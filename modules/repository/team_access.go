// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
)

func CanDoerManageRepoCollaboratorTeam(ctx context.Context, user *user_model.User, repo *repo_model.Repository) (bool, error) {
	if user == nil {
		return false, nil
	}
	if err := repo.LoadOwner(ctx); err != nil {
		return false, err
	}
	if !repo.Owner.IsOrganization() {
		return false, nil
	}

	permission, err := access_model.GetIndividualUserRepoPermission(ctx, repo, user)
	if err != nil {
		return false, err
	}
	return permission.IsOwner() || permission.IsAdmin() && repo.Owner.RepoAdminChangeTeamAccess, nil
}
