// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	"gitea.dev/models/organization"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
)

// CanUserChangeTeamAccess reports whether the user can change team access for the repository.
func CanUserChangeTeamAccess(ctx context.Context, repo *repo_model.Repository, user *user_model.User) (bool, error) {
	if user == nil {
		return false, nil
	}
	if err := repo.LoadOwner(ctx); err != nil {
		return false, err
	}
	if !repo.Owner.IsOrganization() {
		return false, nil
	}
	if user.IsAdmin {
		return true, nil
	}
	permission, err := access_model.GetIndividualUserRepoPermission(ctx, repo, user)
	if err != nil {
		return false, err
	}
	if !permission.IsAdmin() {
		return false, nil
	}
	org := organization.OrgFromUser(repo.Owner)
	if org.RepoAdminChangeTeamAccess {
		return true, nil
	}
	return org.IsOwnedBy(ctx, user.ID)
}
