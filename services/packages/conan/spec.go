// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conan

import (
	conan_module "gitea.dev/modules/packages/conan"
	packages_service "gitea.dev/services/packages"
)

type Specialization struct {
	packages_service.SpecGetViewPackageVersionData
	packages_service.SpecOnBeforeRemovePackageAll
	packages_service.SpecOnBeforeRemovePackageVersion
	packages_service.SpecRenderUsageManual
}

var _ packages_service.Specialization = (*Specialization)(nil)

func (s Specialization) PublicPropertyNames() []string {
	return []string{conan_module.PropertyRecipeUser, conan_module.PropertyRecipeChannel, conan_module.PropertyRecipeRevision, conan_module.PropertyPackageReference, conan_module.PropertyPackageRevision}
}
