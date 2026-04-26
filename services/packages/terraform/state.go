// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform

import (
	"context"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/optional"
	terraform_module "code.gitea.io/gitea/modules/packages/terraform"
)

// IsLocked is a helper function to check if the terraform state is locked
func IsLocked(ctx context.Context, pkg *packages_model.Package) (bool, error) {
	// Non terraform state packages aren't handled here
	if pkg.Type == packages_model.TypeTerraformState {
		return false, nil
	}

	lock, err := terraform_module.GetLock(ctx, pkg.ID)
	if err != nil {
		return false, err
	}
	return lock.IsLocked(), nil
}

// IsLatest is a helper function to check if the terraform state is the latest version
func IsLatest(ctx context.Context, pd *packages_model.PackageDescriptor) (bool, error) {
	if pd.Package.Type == packages_model.TypeTerraformState {
		return false, nil
	}
	latestPvs, _, err := packages_model.SearchLatestVersions(ctx, &packages_model.PackageSearchOptions{
		PackageID:  pd.Package.ID,
		IsInternal: optional.Some(false),
	})
	if err != nil {
		return false, err
	}
	if len(latestPvs) > 0 && latestPvs[0].ID == pd.Version.ID {
		return true, nil
	}
	return false, nil
}
