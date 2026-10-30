// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pkgspec

import (
	packages_model "gitea.dev/models/packages"
	packages_service "gitea.dev/services/packages"
	"gitea.dev/services/packages/alpine"
	"gitea.dev/services/packages/arch"
	"gitea.dev/services/packages/conan"
	"gitea.dev/services/packages/conda"
	"gitea.dev/services/packages/debian"
	"gitea.dev/services/packages/nuget"
	"gitea.dev/services/packages/rpm"
	"gitea.dev/services/packages/terraform"
	"gitea.dev/services/packages/vagrant"
)

func InitManager() error {
	mgr := packages_service.GetSpecManager()
	mgr.Add(packages_model.TypeAlpine, &alpine.Specialization{})
	mgr.Add(packages_model.TypeArch, &arch.Specialization{})
	mgr.Add(packages_model.TypeConan, &conan.Specialization{})
	mgr.Add(packages_model.TypeConda, &conda.Specialization{})
	mgr.Add(packages_model.TypeDebian, &debian.Specialization{})
	mgr.Add(packages_model.TypeNuGet, &nuget.Specialization{})
	mgr.Add(packages_model.TypeRpm, &rpm.Specialization{})
	mgr.Add(packages_model.TypeTerraformState, &terraform.Specialization{})
	mgr.Add(packages_model.TypeVagrant, &vagrant.Specialization{})
	// TODO: add more in the future, refactor the existing code to use this approach
	return nil
}
