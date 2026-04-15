// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
)

func WeekSlice2Map(rows []*repo_model.WeekData) map[int64]*repo_model.WeekData {
	weeks := make(map[int64]*repo_model.WeekData, len(rows))
	for _, row := range rows {
		week := row.Week
		if weeks[week] == nil {
			weeks[week] = &repo_model.WeekData{
				Week: week,
			}
		}
		weeks[week].Commits += row.Commits
		weeks[week].Additions += row.Additions
		weeks[week].Deletions += row.Deletions
	}
	return weeks
}

// GetContributionsOverTime returns weekly contribution totals for the default branch.
func GetContributionsOverTime(ctx context.Context, repo *repo_model.Repository, start, end *time.Time, statTypes ...repo_model.RepoStatType) (map[int64]*repo_model.WeekData, error) {
	if len(statTypes) == 0 {
		return nil, errors.New("no contribution types provided")
	}
	if start != nil && end != nil && !start.Before(*end) {
		return nil, errors.New("invalid contribution range")
	}

	var startDay *repo_model.ContributorDayStart
	var endDay *repo_model.ContributorDayStart
	if start != nil {
		value := repo_model.NewContributorDayStart(start.UTC())
		startDay = &value
	}
	if end != nil {
		value := repo_model.NewContributorDayStart(end.UTC())
		endDay = &value
	}

	weeklyStats, err := repo_model.GetRepoWeeklyStats(ctx, repo.ID, repo_model.StatsOptions{
		Start:     startDay,
		End:       endDay,
		StatTypes: statTypes,
	})
	if err != nil {
		return nil, err
	}
	if len(weeklyStats) == 0 {
		hasStats, err := repo_model.HasRepoContributorDailyStats(ctx, repo.ID)
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
