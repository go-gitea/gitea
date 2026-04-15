// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package contribution

import (
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestGetRepoTopContributors(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repoID := int64(1)
	dayStart := NewContributorDayStart(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	stats := []*ContributorDaily{
		{
			RepoID:      repoID,
			DayStart:    dayStart,
			UserID:      1,
			Email:       "alice@example.com",
			AuthorName:  "Alice",
			Commits:     3,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
		{
			RepoID:      repoID,
			DayStart:    dayStart,
			UserID:      2,
			Email:       "bob@example.com",
			AuthorName:  "Bob",
			Commits:     6,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
		{
			RepoID:      repoID,
			DayStart:    NewContributorDayStart(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)),
			UserID:      1,
			Email:       "alice@example.com",
			AuthorName:  "Alice",
			Commits:     2,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
	}
	_, err := db.GetEngine(t.Context()).Insert(stats)
	assert.NoError(t, err)

	contributors, total, err := GetRepoTopContributors(t.Context(), repoID, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	if assert.Len(t, contributors, 2) {
		assert.Equal(t, "bob@example.com", contributors[0].Email)
		assert.Equal(t, int64(6), contributors[0].Commits)
		assert.Equal(t, "alice@example.com", contributors[1].Email)
		assert.Equal(t, int64(5), contributors[1].Commits)
	}
}

func TestIterateRepoIDsWithoutContributorDaily(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repoWithStats := int64(1)
	_, err := db.GetEngine(t.Context()).Insert(&ContributorDaily{
		RepoID:      repoWithStats,
		DayStart:    NewContributorDayStart(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		UserID:      1,
		Email:       "alice@example.com",
		AuthorName:  "Alice",
		Commits:     1,
		UpdatedUnix: timeutil.TimeStampNow(),
	})
	assert.NoError(t, err)

	collected := make(map[int64]struct{})
	iterateErr := IterateRepoIDsWithoutContributorDaily(t.Context(), 1, func(repoIDs []int64) error {
		for _, repoID := range repoIDs {
			collected[repoID] = struct{}{}
		}
		return nil
	})
	assert.NoError(t, iterateErr)

	_, hasRepoWithStats := collected[repoWithStats]
	assert.False(t, hasRepoWithStats)
}

func TestContributorDayStart(t *testing.T) {
	stamp := NewContributorDayStart(time.Date(2024, 1, 2, 1, 0, 0, 0, time.FixedZone("UTC+8", 8*3600)))
	expected := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	assert.Equal(t, expected, stamp.UnixMilli())
}

func TestGetRepoTopContributorsLimit(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repoID := int64(2)
	dayStart := NewContributorDayStart(time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC))
	_, err := db.GetEngine(t.Context()).Insert([]*ContributorDaily{
		{
			RepoID:      repoID,
			DayStart:    dayStart,
			UserID:      20,
			Email:       "alpha@example.com",
			AuthorName:  "Alpha",
			Commits:     10,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
		{
			RepoID:      repoID,
			DayStart:    dayStart,
			UserID:      21,
			Email:       "beta@example.com",
			AuthorName:  "Beta",
			Commits:     2,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
	})
	assert.NoError(t, err)

	contributors, total, err := GetRepoTopContributors(t.Context(), repoID, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	if assert.Len(t, contributors, 1) {
		assert.Equal(t, "alpha@example.com", contributors[0].Email)
		assert.Equal(t, int64(10), contributors[0].Commits)
	}
}

func TestGetRepoContributorsIncludeAnonymous(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repoID := int64(3)
	dayStart := NewContributorDayStart(time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC))
	_, err := db.GetEngine(t.Context()).Insert([]*ContributorDaily{
		{
			RepoID:      repoID,
			DayStart:    dayStart,
			UserID:      10,
			Email:       "known@example.com",
			AuthorName:  "Known",
			Commits:     4,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
		{
			RepoID:      repoID,
			DayStart:    dayStart,
			UserID:      0,
			Email:       "anon@example.com",
			AuthorName:  "Anon",
			Commits:     3,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
	})
	assert.NoError(t, err)

	contributors, total, err := GetRepoContributors(t.Context(), repoID, false, db.ListOptions{PageSize: 10, Page: 1})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, contributors, 1)
	assert.Equal(t, int64(10), contributors[0].UserID)

	contributors, total, err = GetRepoContributors(t.Context(), repoID, true, db.ListOptions{PageSize: 10, Page: 1})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, contributors, 2)
}

func TestGetRepoWeeklyStatsCodeFrequency(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repoID := int64(4)
	weekStart := time.Date(2024, 3, 3, 0, 0, 0, 0, time.UTC) // Sunday
	otherWeek := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	_, err := db.GetEngine(t.Context()).Insert([]*ContributorDaily{
		{
			RepoID:      repoID,
			DayStart:    NewContributorDayStart(weekStart.Add(24 * time.Hour)),
			UserID:      1,
			Email:       "alpha@example.com",
			AuthorName:  "Alpha",
			Additions:   5,
			Deletions:   2,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
		{
			RepoID:      repoID,
			DayStart:    NewContributorDayStart(weekStart.Add(48 * time.Hour)),
			UserID:      2,
			Email:       "beta@example.com",
			AuthorName:  "Beta",
			Additions:   3,
			Deletions:   1,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
		{
			RepoID:      repoID,
			DayStart:    NewContributorDayStart(otherWeek),
			UserID:      3,
			Email:       "gamma@example.com",
			AuthorName:  "Gamma",
			Additions:   2,
			Deletions:   0,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
	})
	assert.NoError(t, err)

	rows, err := GetRepoWeeklyStats(t.Context(), repoID, StatsOptions{
		StatTypes: []RepoStatType{RepoStatAdditions, RepoStatDeletions},
	})
	assert.NoError(t, err)
	if assert.Len(t, rows, 2) {
		assert.Equal(t, weekStart.UnixMilli(), rows[0].Week)
		assert.Equal(t, int64(8), rows[0].Additions)
		assert.Equal(t, int64(3), rows[0].Deletions)
		assert.Equal(t, otherWeek.UnixMilli(), rows[1].Week)
		assert.Equal(t, int64(2), rows[1].Additions)
		assert.Equal(t, int64(0), rows[1].Deletions)
	}
}

func TestGetRepoWeeklyStatsCommitCounts(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repoID := int64(5)
	weekStart := time.Date(2024, 5, 5, 0, 0, 0, 0, time.UTC)
	otherWeek := time.Date(2024, 5, 12, 0, 0, 0, 0, time.UTC)
	_, err := db.GetEngine(t.Context()).Insert([]*ContributorDaily{
		{
			RepoID:      repoID,
			DayStart:    NewContributorDayStart(weekStart.Add(24 * time.Hour)),
			UserID:      1,
			Email:       "alpha@example.com",
			AuthorName:  "Alpha",
			Commits:     2,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
		{
			RepoID:      repoID,
			DayStart:    NewContributorDayStart(weekStart.Add(48 * time.Hour)),
			UserID:      2,
			Email:       "beta@example.com",
			AuthorName:  "Beta",
			Commits:     3,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
		{
			RepoID:      repoID,
			DayStart:    NewContributorDayStart(otherWeek),
			UserID:      3,
			Email:       "gamma@example.com",
			AuthorName:  "Gamma",
			Commits:     4,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
	})
	assert.NoError(t, err)

	rows, err := GetRepoWeeklyStats(t.Context(), repoID, StatsOptions{
		StatTypes: []RepoStatType{RepoStatCommits},
	})
	assert.NoError(t, err)
	if assert.Len(t, rows, 2) {
		assert.Equal(t, weekStart.UnixMilli(), rows[0].Week)
		assert.Equal(t, int64(5), rows[0].Commits)
		assert.Equal(t, otherWeek.UnixMilli(), rows[1].Week)
		assert.Equal(t, int64(4), rows[1].Commits)
	}
}

func TestGetContributorActivity(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	firstDay := NewContributorDayStart(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	secondDay := NewContributorDayStart(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	_, err := db.GetEngine(t.Context()).Insert([]*ContributorDaily{
		{
			RepoID:      repo.ID,
			DayStart:    firstDay,
			UserID:      0,
			Email:       "alpha@example.com",
			AuthorName:  "Alpha",
			Additions:   4,
			Deletions:   1,
			Commits:     2,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
		{
			RepoID:      repo.ID,
			DayStart:    secondDay,
			UserID:      0,
			Email:       "alpha@example.com",
			AuthorName:  "Alpha",
			Additions:   3,
			Deletions:   2,
			Commits:     1,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
		{
			RepoID:      repo.ID,
			DayStart:    firstDay,
			UserID:      0,
			Email:       "beta@example.com",
			AuthorName:  "Beta",
			Additions:   1,
			Deletions:   0,
			Commits:     1,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
	})
	assert.NoError(t, err)

	rows, err := GetContributorActivity(t.Context(), repo, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 10)
	assert.NoError(t, err)
	assert.Len(t, rows, 2)

	var alpha *ContributorSummary
	for _, row := range rows {
		if row.Email == "alpha@example.com" {
			alpha = row
		}
	}
	if assert.NotNil(t, alpha) {
		assert.Equal(t, int64(7), alpha.Additions)
		assert.Equal(t, int64(3), alpha.Deletions)
		assert.Equal(t, int64(3), alpha.Commits)
	}
}
