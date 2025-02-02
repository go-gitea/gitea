// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"context"
	"fmt"

	org_model "code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
)

func LinkToRepository(ctx context.Context, pkg *packages_model.Package, repo *repo_model.Repository, doer *user_model.User) error {
	if pkg.OwnerID != repo.OwnerID {
		return util.ErrNotExist
	}

	perms, err := access_model.GetUserRepoPermission(ctx, repo, doer)
	if err != nil {
		return fmt.Errorf("error getting permissions for user %d on repository %d: %w", doer.ID, repo.ID, err)
	}

	if !perms.CanWrite(unit.TypePackages) {
		return fmt.Errorf("no permission to link this package and repository, or packages are disabled")
	}

	if err := packages_model.SetRepositoryLink(ctx, pkg.ID, repo.ID); err != nil {
		return fmt.Errorf("error updating package: %w", err)
	}
	return nil
}

func UnlinkFromRepository(ctx context.Context, pkg *packages_model.Package, doer *user_model.User) error {
	user, err := user_model.GetUserByID(ctx, pkg.OwnerID)
	if err != nil {
		return err
	}
	if !user.IsAdmin {
		if !user.IsOrganization() {
			if doer.ID != pkg.OwnerID {
				return fmt.Errorf("No permission to unlink this package and repository, or packages are disabled")
			}
		} else {
			isOrgAdmin, err := org_model.OrgFromUser(user).IsOrgAdmin(ctx, doer.ID)
			if err != nil {
				return err
			} else if !isOrgAdmin {
				return fmt.Errorf("No permission to unlink this package and repository, or packages are disabled")
			}
		}
	}
	return packages_model.UnlinkRepository(ctx, pkg.ID)
}
