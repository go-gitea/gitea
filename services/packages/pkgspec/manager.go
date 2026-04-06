// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pkgspec

import (
	packages_model "code.gitea.io/gitea/models/packages"
	packages_service "code.gitea.io/gitea/services/packages"
	"code.gitea.io/gitea/services/packages/terraform"
)

func InitManager() error {
	packages_service.GetSpecManager().Add(packages_model.TypeTerraformState, &terraform.Specialization{})
	return nil
}
