// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"context"

	packages_model "gitea.dev/models/packages"
	arch_module "gitea.dev/modules/packages/arch"
)

// GetRepositories gets all available repositories (team_id = 0)
func GetRepositories(ctx context.Context, ownerID int64) ([]string, error) {
	return GetTeamRepositories(ctx, ownerID, 0)
}

// GetTeamRepositories gets all available repositories for a team
func GetTeamRepositories(ctx context.Context, ownerID, teamID int64) ([]string, error) {
	return packages_model.GetDistinctPropertyValuesForTeam(
		ctx,
		packages_model.TypeArch,
		ownerID,
		teamID,
		packages_model.PropertyTypeFile,
		arch_module.PropertyRepository,
		nil,
	)
}

// GetArchitectures gets all available architectures for the given repository (team_id = 0)
func GetArchitectures(ctx context.Context, ownerID int64, repository string) ([]string, error) {
	return GetTeamArchitectures(ctx, ownerID, 0, repository)
}

// GetTeamArchitectures gets all available architectures for the given repository and team
func GetTeamArchitectures(ctx context.Context, ownerID, teamID int64, repository string) ([]string, error) {
	return packages_model.GetDistinctPropertyValuesForTeam(
		ctx,
		packages_model.TypeArch,
		ownerID,
		teamID,
		packages_model.PropertyTypeFile,
		arch_module.PropertyArchitecture,
		&packages_model.DistinctPropertyDependency{
			Name:  arch_module.PropertyRepository,
			Value: repository,
		},
	)
}
