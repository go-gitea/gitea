// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"context"

	packages_model "code.gitea.io/gitea/models/packages"
	arch_module "code.gitea.io/gitea/modules/packages/arch"
)

// GetRepositories gets all available repositories
func GetRepositories(ctx context.Context, ownerID int64) ([]string, error) {
	return packages_model.GetDistinctPropertyValues(
		ctx,
		packages_model.TypeArch,
		ownerID,
		packages_model.PropertyTypeFile,
		arch_module.PropertyRepository,
		nil,
	)
}

// GetArchitectures gets all available architectures for the given repository
func GetArchitectures(ctx context.Context, ownerID int64, repository string) ([]string, error) {
	return packages_model.GetDistinctPropertyValues(
		ctx,
		packages_model.TypeArch,
		ownerID,
		packages_model.PropertyTypeFile,
		arch_module.PropertyArchitecture,
		&packages_model.DistinctPropertyDependency{
			Name:  arch_module.PropertyRepository,
			Value: repository,
		},
	)
}
