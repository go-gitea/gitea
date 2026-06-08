// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform_module

import (
	"context"

	packages_model "gitea.dev/models/packages"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"
	packages_service "gitea.dev/services/packages"
)

// Specialization implements packages_service.Specialization for the
// terraform-module package type. Unlike the Terraform State package
// there are no lock semantics: modules are immutable releases and any
// version may be deleted by a writer.
type Specialization struct{}

var _ packages_service.Specialization = (*Specialization)(nil)

// ViewData is exposed to the web UI as PackageVersionViewData.
type ViewData struct {
	IsLatestVersion bool
}

// GetViewPackageVersionData annotates the package version with whether
// it is the latest known version (used by the UI to surface a "latest"
// badge next to the install snippet).
func (Specialization) GetViewPackageVersionData(ctx context.Context, pd *packages_model.PackageDescriptor) (any, error) {
	out := ViewData{}
	latestPvs, _, err := packages_model.SearchLatestVersions(ctx, &packages_model.PackageSearchOptions{
		PackageID:  pd.Package.ID,
		IsInternal: optional.Some(false),
	})
	if err != nil {
		return out, err
	}
	out.IsLatestVersion = len(latestPvs) > 0 && latestPvs[0].ID == pd.Version.ID
	return out, nil
}

// OnBeforeRemovePackageAll has no extra constraints for modules.
func (Specialization) OnBeforeRemovePackageAll(_ context.Context, _ *user_model.User, _ *packages_model.Package, _ []*packages_model.PackageDescriptor) error {
	return nil
}

// OnBeforeRemovePackageVersion has no extra constraints: any version may be deleted.
func (Specialization) OnBeforeRemovePackageVersion(_ context.Context, _ *user_model.User, _ *packages_model.PackageDescriptor) error {
	return nil
}
