// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cran

import (
	cran_module "gitea.dev/modules/packages/cran"
	packages_service "gitea.dev/services/packages"
)

type Specialization struct {
	packages_service.SpecGetViewPackageVersionData
	packages_service.SpecOnBeforeRemovePackageAll
	packages_service.SpecOnBeforeRemovePackageVersion
	packages_service.SpecRenderUsageManual
}

var _ packages_service.Specialization = (*Specialization)(nil)

// PublicPropertyNames are the properties that tell a source package from a
// binary one and, for a binary, the platform and R version it was built for.
func (s Specialization) PublicPropertyNames() []string {
	return []string{cran_module.PropertyType, cran_module.PropertyPlatform, cran_module.PropertyRVersion}
}
