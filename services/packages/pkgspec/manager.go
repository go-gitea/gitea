// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pkgspec

import (
	packages_model "gitea.dev/models/packages"
	packages_service "gitea.dev/services/packages"
	"gitea.dev/services/packages/terraform"
	terraform_module "gitea.dev/services/packages/terraform_module"
)

func InitManager() error {
	mgr := packages_service.GetSpecManager()
	mgr.Add(packages_model.TypeTerraformState, &terraform.Specialization{})
	mgr.Add(packages_model.TypeTerraformModule, &terraform_module.Specialization{})
	// TODO: add more in the future, refactor the existing code to use this approach
	return nil
}
