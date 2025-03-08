// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"context"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
)

// GetUserHeatmapDataByUser returns an array of UserHeatmapData
func GetUserHeatmapDataByUser(ctx context.Context, user, doer *user_model.User) ([]*activities_model.UserHeatmapData, error) {
	return activities_model.GetUserHeatmapData(ctx, user, nil, doer)
}

// GetUserHeatmapDataByUserTeam returns an array of UserHeatmapData
func GetUserHeatmapDataByUserTeam(ctx context.Context, user *user_model.User, team *organization.Team, doer *user_model.User) ([]*activities_model.UserHeatmapData, error) {
	return activities_model.GetUserHeatmapData(ctx, user, team, doer)
}
