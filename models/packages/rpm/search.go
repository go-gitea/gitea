// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rpm

import (
	"context"

	packages_model "gitea.dev/models/packages"
	rpm_module "gitea.dev/modules/packages/rpm"
)

// GetGroups gets all available groups (team_id = 0)
func GetGroups(ctx context.Context, ownerID int64) ([]string, error) {
	return GetTeamGroups(ctx, ownerID, 0)
}

// GetTeamGroups gets all available groups for a team
func GetTeamGroups(ctx context.Context, ownerID, teamID int64) ([]string, error) {
	return packages_model.GetDistinctPropertyValuesForTeam(
		ctx,
		packages_model.TypeRpm,
		ownerID,
		teamID,
		packages_model.PropertyTypeFile,
		rpm_module.PropertyGroup,
		nil,
	)
}
