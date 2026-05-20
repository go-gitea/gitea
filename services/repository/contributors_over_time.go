// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	contribution_model "code.gitea.io/gitea/models/repo/contribution"
)

func WeekSlice2Map(rows []*contribution_model.WeekData) map[int64]*contribution_model.WeekData {
	weeks := make(map[int64]*contribution_model.WeekData, len(rows))
	for _, row := range rows {
		week := row.Week
		if weeks[week] == nil {
			weeks[week] = &contribution_model.WeekData{
				Week: week,
			}
		}
		weeks[week].Commits += row.Commits
		weeks[week].Additions += row.Additions
		weeks[week].Deletions += row.Deletions
		weeks[week].ChangedFiles += row.ChangedFiles
	}
	return weeks
}

// GetContributionsOverTime returns weekly contribution totals for the default branch.
func GetContributionsOverTime(ctx context.Context, repo *repo_model.Repository, start, end *time.Time, statTypes ...contribution_model.RepoStatType) (map[int64]*contribution_model.WeekData, error) {
	if len(statTypes) == 0 {
		return nil, errors.New("no contribution types provided")
	}
	if start != nil && end != nil && !start.Before(*end) {
		return nil, errors.New("invalid contribution range")
	}

	var startDay *contribution_model.ContributorDayStart
	var endDay *contribution_model.ContributorDayStart
	if start != nil {
		value := contribution_model.NewContributorDayStart(start.UTC())
		startDay = &value
	}
	if end != nil {
		value := contribution_model.NewContributorDayStart(end.UTC())
		endDay = &value
	}

	weeklyStats, err := contribution_model.GetRepoWeeklyStats(ctx, repo.ID, contribution_model.StatsOptions{
		Start:     startDay,
		End:       endDay,
		StatTypes: statTypes,
	})
	if err != nil {
		return nil, err
	}
	if len(weeklyStats) == 0 {
		hasStats, err := contribution_model.HasRepoContributorDailyStats(ctx, repo.ID)
		if err != nil {
			return nil, err
		}
		if !hasStats {
			if err := RequestContributorStatsRebuild(ctx, repo.ID); err != nil {
				return nil, err
			}
			return nil, ErrAwaitGeneration
		}
	}

	return WeekSlice2Map(weeklyStats), nil
}
