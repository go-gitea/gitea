// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// UserHeatmapData represents the data needed to create a heatmap
type UserHeatmapData struct {
	Timestamp     timeutil.TimeStamp `json:"timestamp"`
	Contributions int64              `json:"contributions"`
}

// GetUserHeatmapDataByUser returns an array of UserHeatmapData
func GetUserHeatmapDataByUser(ctx context.Context, user, doer *user_model.User) ([]*UserHeatmapData, error) {
	return getUserHeatmapData(ctx, user, nil, doer)
}

// GetUserHeatmapDataByUserTeam returns an array of UserHeatmapData
func GetUserHeatmapDataByUserTeam(ctx context.Context, user *user_model.User, team *organization.Team, doer *user_model.User) ([]*UserHeatmapData, error) {
	return getUserHeatmapData(ctx, user, team, doer)
}

func getUserHeatmapData(ctx context.Context, user *user_model.User, team *organization.Team, doer *user_model.User) ([]*UserHeatmapData, error) {
	hdata := make([]*UserHeatmapData, 0)

	if !ActivityReadable(user, doer) {
		return hdata, nil
	}

	// Group by 15 minute intervals which will allow the client to accurately shift the timestamp to their timezone.
	// The interval is based on the fact that there are timezones such as UTC +5:30 and UTC +12:45.
	groupBy := "commit_timestamp / 900 * 900"
	groupByName := "timestamp" // We need this extra case because mssql doesn't allow grouping by alias
	switch {
	case setting.Database.Type.IsMySQL():
		groupBy = "commit_timestamp DIV 900 * 900"
	case setting.Database.Type.IsMSSQL():
		groupByName = groupBy
	}

	cutoff := timeutil.TimeStampNow() - (366+7)*86400 // (366+7) days to include the first week for the heatmap

	cond := builder.NewCond()
	cond = cond.And(builder.Eq{"user_id": user.ID})
	cond = cond.And(builder.Gt{"commit_timestamp": cutoff})

	// Filter by accessible repos for the viewer
	if doer == nil || !doer.IsAdmin {
		cond = cond.And(builder.In("repo_id", repo_model.AccessibleRepoIDsQuery(doer)))
	}

	// Filter by team repos if applicable
	if team != nil {
		env := repo_model.AccessibleTeamReposEnv(organization.OrgFromUser(user), team)
		teamRepoIDs, err := env.RepoIDs(ctx)
		if err != nil {
			return nil, err
		}
		cond = cond.And(builder.In("repo_id", teamRepoIDs))
	}

	return hdata, db.GetEngine(ctx).
		Table("user_heatmap_commit").
		Select(groupBy+" AS timestamp, count(*) as contributions").
		Where(cond).
		GroupBy(groupByName).
		OrderBy("timestamp").
		Find(&hdata)
}

// GetTotalContributionsInHeatmap returns the total number of contributions in a heatmap
func GetTotalContributionsInHeatmap(hdata []*UserHeatmapData) int64 {
	var total int64
	for _, v := range hdata {
		total += v.Contributions
	}
	return total
}
