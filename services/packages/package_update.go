// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"context"
	"fmt"

	org_model "gitea.dev/models/organization"
	packages_model "gitea.dev/models/packages"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/util"
)

func LinkToRepository(ctx context.Context, pkg *packages_model.Package, repo *repo_model.Repository, doer *user_model.User) error {
	if pkg.OwnerID != repo.OwnerID {
		return util.ErrPermissionDenied
	}
	if pkg.RepoID > 0 {
		return util.ErrInvalidArgument
	}

	perms, err := access_model.GetDoerRepoPermission(ctx, repo, doer)
	if err != nil {
		return fmt.Errorf("error getting permissions for user %d on repository %d: %w", doer.ID, repo.ID, err)
	}
	if !perms.CanWrite(unit.TypePackages) {
		return util.ErrPermissionDenied
	}

	if err := packages_model.SetRepositoryLink(ctx, pkg.ID, repo.ID); err != nil {
		return fmt.Errorf("error while linking package '%v' to repo '%v' : %w", pkg.Name, repo.FullName(), err)
	}
	return nil
}

func canDoerManagePackage(ctx context.Context, pkg *packages_model.Package, doer *user_model.User) bool {
	owner, err := user_model.GetUserByID(ctx, pkg.OwnerID)
	if err != nil {
		return false
	}
	if doer.IsAdmin {
		return true
	}
	if !owner.IsOrganization() {
		return doer.ID == pkg.OwnerID
	}

	teams, _ := org_model.GetUserOrgTeams(ctx, owner.ID, doer.ID)
	return teams.HasAllRepoAdminAccess()
}

func UnlinkFromRepository(ctx context.Context, pkg *packages_model.Package, doer *user_model.User) error {
	if pkg.RepoID == 0 {
		return util.ErrInvalidArgument
	}
	if !canDoerManagePackage(ctx, pkg, doer) {
		return util.ErrPermissionDenied
	}
	return packages_model.UnlinkRepository(ctx, pkg.ID)
}
