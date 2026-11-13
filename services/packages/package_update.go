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

// UpdateRepositoryLink links the package to repo, replacing an existing link, or removes the link if repo is nil
func UpdateRepositoryLink(ctx context.Context, pkg *packages_model.Package, repo *repo_model.Repository, doer *user_model.User) error {
	if repo == nil {
		if pkg.RepoID == 0 {
			return nil
		}
		return UnlinkFromRepository(ctx, pkg, doer)
	}
	if pkg.OwnerID != repo.OwnerID {
		return util.ErrPermissionDenied
	}
	perms, err := access_model.GetDoerRepoPermission(ctx, repo, doer)
	if err != nil {
		return fmt.Errorf("error getting permissions for user %d on repository %d: %w", doer.ID, repo.ID, err)
	}
	if !perms.CanWrite(unit.TypePackages) {
		return util.ErrPermissionDenied
	}
	if pkg.RepoID == repo.ID {
		return nil
	}
	if pkg.RepoID != 0 && !canDoerManagePackage(ctx, pkg, doer) {
		return util.ErrPermissionDenied
	}
	return packages_model.SetRepositoryLink(ctx, pkg.ID, repo.ID)
}

func LinkToRepository(ctx context.Context, pkg *packages_model.Package, repo *repo_model.Repository, doer *user_model.User) error {
	if pkg.RepoID > 0 {
		return util.ErrInvalidArgument
	}
	return UpdateRepositoryLink(ctx, pkg, repo, doer)
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
	if teams.HasAllRepoAdminAccess() {
		return true
	}

	if pkg.RepoID == 0 {
		return false
	}
	// old behavior: repo admin can manage the package linked to the repo
	repo, err := repo_model.GetRepositoryByID(ctx, pkg.RepoID)
	if err != nil {
		return false
	}
	perms, err := access_model.GetDoerRepoPermission(ctx, repo, doer)
	return err == nil && perms.IsAdmin()
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
