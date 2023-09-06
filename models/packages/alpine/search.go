// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package alpine

import (
	"context"

	packages_model "code.gitea.io/gitea/models/packages"
	alpine_module "code.gitea.io/gitea/modules/packages/alpine"
)

// GetBranches gets all available branches
func GetBranches(ctx context.Context, ownerID int64) ([]string, error) {
	return packages_model.GetDistinctPropertyValues(
		ctx,
		packages_model.TypeAlpine,
		ownerID,
		packages_model.PropertyTypeFile,
		alpine_module.PropertyBranch,
		nil,
	)
}

// GetRepositories gets all available repositories for the given branch
func GetRepositories(ctx context.Context, ownerID int64, branch string) ([]string, error) {
	return packages_model.GetDistinctPropertyValues(
		ctx,
		packages_model.TypeAlpine,
		ownerID,
		packages_model.PropertyTypeFile,
		alpine_module.PropertyRepository,
		&packages_model.DistinctPropertyDependency{
			Name:  alpine_module.PropertyBranch,
			Value: branch,
		},
	)
}

// GetArchitectures gets all available architectures for the given repository
func GetArchitectures(ctx context.Context, ownerID int64, repository string) ([]string, error) {
	return packages_model.GetDistinctPropertyValues(
		ctx,
		packages_model.TypeAlpine,
		ownerID,
		packages_model.PropertyTypeFile,
		alpine_module.PropertyArchitecture,
		&packages_model.DistinctPropertyDependency{
			Name:  alpine_module.PropertyRepository,
			Value: repository,
		},
	)
}
