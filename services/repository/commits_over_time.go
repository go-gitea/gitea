// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
)

// WeeklyCommitData represents weekly commit counts for a repository.
type WeeklyCommitData struct {
	Week    int64 `json:"week"`
	Commits int64 `json:"commits"`
}

// GetCommitsOverTime returns weekly commit totals for the default branch.
func GetCommitsOverTime(ctx context.Context, repo *repo_model.Repository) ([]*WeeklyCommitData, error) {
	stats, err := repo_model.GetRepoContributorDailyStats(ctx, repo.ID)
	if err != nil {
		return nil, err
	}
	if len(stats) == 0 {
		if err := RequestContributorStatsRebuild(ctx, repo.ID); err != nil {
			return nil, err
		}
		return nil, ErrAwaitGeneration
	}

	weeklyTotals := make(map[int64]int64)
	var minWeek int64
	var maxWeek int64
	for _, stat := range stats {
		week := weekStartUnixMilliFromDayStart(stat.DayStart)
		weeklyTotals[week] += stat.Commits
		if minWeek == 0 || week < minWeek {
			minWeek = week
		}
		if week > maxWeek {
			maxWeek = week
		}
	}
	if minWeek == 0 {
		return nil, nil
	}

	nowWeek := weekStartUnixMilliFromTime(time.Now().UTC())
	if nowWeek > maxWeek {
		maxWeek = nowWeek
	}

	res := make([]*WeeklyCommitData, 0, int((maxWeek-minWeek)/contributorWeekMillis)+1)
	for week := minWeek; week <= maxWeek; week += contributorWeekMillis {
		res = append(res, &WeeklyCommitData{
			Week:    week,
			Commits: weeklyTotals[week],
		})
	}
	return res, nil
}
