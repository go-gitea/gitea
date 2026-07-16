// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
)

// UserHeatmapData represents the data needed to create a heatmap
type UserHeatmapData struct {
	Timestamp     timeutil.TimeStamp `json:"timestamp"`
	Contributions int64              `json:"contributions"`
}

// GetUserHeatmapDataByUser returns an array of UserHeatmapData, it checks whether doer can access user's activity
func GetUserHeatmapDataByUser(ctx context.Context, user, doer *user_model.User, year int, showPrivate bool) ([]*UserHeatmapData, error) {
	return getUserHeatmapData(ctx, user, nil, doer, year, showPrivate)
}

// GetUserHeatmapDataByOrgTeam returns an array of UserHeatmapData, it checks whether doer can access org's activity
func GetUserHeatmapDataByOrgTeam(ctx context.Context, org *organization.Organization, team *organization.Team, doer *user_model.User, year int, showPrivate bool) ([]*UserHeatmapData, error) {
	return getUserHeatmapData(ctx, org.AsUser(), team, doer, year, showPrivate)
}

func getUserHeatmapData(ctx context.Context, user *user_model.User, team *organization.Team, doer *user_model.User, year int, showPrivate bool) ([]*UserHeatmapData, error) {
	hdata := make([]*UserHeatmapData, 0)

	if !ActivityReadable(user, doer) {
		return hdata, nil
	}

	// Group by 15 minute intervals which will allow the client to accurately shift the timestamp to their timezone.
	// The interval is based on the fact that there are timezones such as UTC +5:30 and UTC +12:45.
	groupBy := "created_unix / 900 * 900"
	groupByName := "timestamp" // We need this extra case because mssql doesn't allow grouping by alias
	switch {
	case setting.Database.Type.IsMySQL():
		groupBy = "created_unix DIV 900 * 900"
	case setting.Database.Type.IsMSSQL():
		groupByName = groupBy
	}

	cond, err := ActivityQueryCondition(ctx, GetFeedsOptions{
		RequestedUser:  user,
		RequestedTeam:  team,
		Actor:          doer,
		IncludePrivate: showPrivate,
		IncludeDeleted: true,
		// * Heatmaps for individual users only include actions that the user themself did.
		// * For organizations actions by all users that were made in owned
		//   repositories are counted.
		OnlyPerformedBy: !user.IsOrganization(),
	})
	if err != nil {
		return nil, err
	}

	var startTime, endTime int64
	if year > 0 {
		loc := setting.DefaultUILocation
		start := time.Date(year-1, time.December, 25, 0, 0, 0, 0, loc)
		end := time.Date(year, time.December, 31, 23, 59, 59, 999999999, loc)
		startTime = start.Unix()
		endTime = end.Unix()
	} else {
		startTime = int64(timeutil.TimeStampNow() - (366+7)*86400)
	}

	query := db.GetEngine(ctx).
		Select(groupBy+" AS timestamp, count(user_id) as contributions").
		Table("action").
		Where(cond).
		And("created_unix > ?", startTime)

	if endTime > 0 {
		query = query.And("created_unix <= ?", endTime)
	}

	// HINT: USER-ACTIVITY-PUSH-COMMITS: it only uses the doer's action time, it doesn't use git commit's time
	return hdata, query.
		GroupBy(groupByName).
		OrderBy("timestamp").
		Find(&hdata)
}
