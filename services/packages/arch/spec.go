// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"context"

	packages_model "gitea.dev/models/packages"
	arch_module "gitea.dev/modules/packages/arch"
	packages_service "gitea.dev/services/packages"
)

type Specialization struct {
	packages_service.SpecGetViewPackageVersionData
	packages_service.SpecOnBeforeRemovePackageAll
	packages_service.SpecOnBeforeRemovePackageVersion
	packages_service.SpecRenderUsageManual
}

var _ packages_service.Specialization = (*Specialization)(nil)

func (s Specialization) OnAfterRemovePackage(ctx context.Context, pkg *packages_model.Package) error {
	release, err := AcquireRegistryLock(ctx, pkg.OwnerID)
	if err != nil {
		return err
	}
	defer release()
	return BuildAllRepositoryFiles(ctx, pkg.OwnerID)
}

func (s Specialization) PublicPropertyNames() []string {
	return []string{arch_module.PropertyRepository, arch_module.PropertyArchitecture}
}
