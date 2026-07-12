// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"

	"xorm.io/builder"
)

// UserHeatmapData represents the data needed to create a heatmap
type UserHeatmapData struct {
	Timestamp     timeutil.TimeStamp `json:"timestamp"`
	Contributions int64              `json:"contributions"`
}

// GetUserHeatmapDataByUser returns an array of UserHeatmapData, it checks whether doer can access user's activity
func GetUserHeatmapDataByUser(ctx context.Context, user, doer *user_model.User) ([]*UserHeatmapData, error) {
	return getUserHeatmapData(ctx, user, nil, doer)
}

// GetUserHeatmapDataByOrgTeam returns an array of UserHeatmapData, it checks whether doer can access org's activity
func GetUserHeatmapDataByOrgTeam(ctx context.Context, org *organization.Organization, team *organization.Team, doer *user_model.User) ([]*UserHeatmapData, error) {
	return getUserHeatmapData(ctx, org.AsUser(), team, doer)
}

func getUserHeatmapData(ctx context.Context, user *user_model.User, team *organization.Team, doer *user_model.User) ([]*UserHeatmapData, error) {
	hdata := make([]*UserHeatmapData, 0)

	if !ActivityReadable(user, doer) {
		return hdata, nil
	}

	// Group by 15 minute intervals which will allow the client to accurately shift the timestamp to their timezone.
	// The interval is based on the fact that there are timezones such as UTC +5:30 and UTC +12:45.
	groupBy, groupByName := heatmapGroupBy()

	var cond builder.Cond
	if !user.IsOrganization() && user.ShowPrivateActivity && user_model.IsUserVisibleToViewer(ctx, user, doer) {
		// the user opted in to counting private contributions publicly, so skip
		// the repo-access filtering and count all their own actions (same shape
		// as the owner fast path in GetFeeds); ActivityReadable is checked above,
		// and the visibility check keeps limited/private profiles hidden from
		// viewers who cannot see the profile at all
		cond = builder.Eq{"user_id": user.ID, "act_user_id": user.ID}
	} else {
		var err error
		cond, err = ActivityQueryCondition(ctx, GetFeedsOptions{
			RequestedUser:  user,
			RequestedTeam:  team,
			Actor:          doer,
			IncludePrivate: true, // don't filter by private, as we already filter by repo access
			IncludeDeleted: true,
			// * Heatmaps for individual users only include actions that the user themself did.
			// * For organizations actions by all users that were made in owned
			//   repositories are counted.
			OnlyPerformedBy: !user.IsOrganization(),
		})
		if err != nil {
			return nil, err
		}
	}

	// HINT: USER-ACTIVITY-PUSH-COMMITS: it only uses the doer's action time, it doesn't use git commit's time
	return hdata, db.GetEngine(ctx).
		Select(groupBy+" AS timestamp, count(user_id) as contributions").
		Table("action").
		Where(cond).
		And("created_unix > ?", timeutil.TimeStampNow()-(366+7)*86400). // (366+7) days to include the first week for the heatmap
		GroupBy(groupByName).
		OrderBy("timestamp").
		Find(&hdata)
}

// heatmapGroupBy returns the SQL expression and the identifier to GROUP BY on
// for bucketing actions into 15-minute intervals. The name is an alias
// everywhere except MSSQL, which does not allow grouping by an alias.
func heatmapGroupBy() (groupBy, groupByName string) {
	groupBy = "created_unix / 900 * 900"
	groupByName = "timestamp"
	switch {
	case setting.Database.Type.IsMySQL():
		groupBy = "created_unix DIV 900 * 900"
	case setting.Database.Type.IsMSSQL():
		groupByName = groupBy
	}
	return groupBy, groupByName
}
