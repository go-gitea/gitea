// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"context"

	packages_model "code.gitea.io/gitea/models/packages"
	user_model "code.gitea.io/gitea/models/user"
	packages_service "code.gitea.io/gitea/services/packages"
	alpine_service "code.gitea.io/gitea/services/packages/alpine"
	cargo_service "code.gitea.io/gitea/services/packages/cargo"
	debian_service "code.gitea.io/gitea/services/packages/debian"
	rpm_service "code.gitea.io/gitea/services/packages/rpm"
)

// RemovePackageVersionByNameAndVersion deletes a package version and all associated files
func RemovePackageVersionByNameAndVersion(ctx context.Context, doer *user_model.User, pvi *packages_service.PackageInfo) error {
	pv, err := packages_model.GetVersionByNameAndVersion(ctx, pvi.Owner.ID, pvi.PackageType, pvi.Name, pvi.Version)
	if err != nil {
		return err
	}

	return RemovePackageVersion(ctx, doer, pv)
}

func RemovePackageVersion(ctx context.Context, doer *user_model.User, pv *packages_model.PackageVersion) error {
	pd, err := packages_model.GetPackageDescriptor(ctx, pv)
	if err != nil {
		return err
	}
	switch pd.Package.Type {
	case packages_model.TypeAlpine:
		return alpine_service.RemovePackageVersion(ctx, doer, pv)
	case packages_model.TypeCargo:
		return cargo_service.RemovePackageVersion(ctx, doer, pv)
	case packages_model.TypeDebian:
		return debian_service.RemovePackageVersion(ctx, doer, pv)
	case packages_model.TypeRpm:
		return rpm_service.RemovePackageVersion(ctx, doer, pv)
	default:
		return packages_service.RemovePackageVersion(ctx, doer, pv)
	}
}
