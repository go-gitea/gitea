// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package alpine

import (
	"context"

	packages_model "gitea.dev/models/packages"
	alpine_module "gitea.dev/modules/packages/alpine"
)

// GetBranches gets all available branches (team_id = 0)
func GetBranches(ctx context.Context, ownerID int64) ([]string, error) {
	return GetTeamBranches(ctx, ownerID, 0)
}

// GetTeamBranches gets all available branches for a team
func GetTeamBranches(ctx context.Context, ownerID, teamID int64) ([]string, error) {
	return packages_model.GetDistinctPropertyValuesForTeam(
		ctx,
		packages_model.TypeAlpine,
		ownerID,
		teamID,
		packages_model.PropertyTypeFile,
		alpine_module.PropertyBranch,
		nil,
	)
}

// GetRepositories gets all available repositories for the given branch (team_id = 0)
func GetRepositories(ctx context.Context, ownerID int64, branch string) ([]string, error) {
	return GetTeamRepositories(ctx, ownerID, 0, branch)
}

// GetTeamRepositories gets all available repositories for the given branch and team
func GetTeamRepositories(ctx context.Context, ownerID, teamID int64, branch string) ([]string, error) {
	return packages_model.GetDistinctPropertyValuesForTeam(
		ctx,
		packages_model.TypeAlpine,
		ownerID,
		teamID,
		packages_model.PropertyTypeFile,
		alpine_module.PropertyRepository,
		&packages_model.DistinctPropertyDependency{
			Name:  alpine_module.PropertyBranch,
			Value: branch,
		},
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
		packages_model.TypeAlpine,
		ownerID,
		teamID,
		packages_model.PropertyTypeFile,
		alpine_module.PropertyArchitecture,
		&packages_model.DistinctPropertyDependency{
			Name:  alpine_module.PropertyRepository,
			Value: repository,
		},
	)
}
