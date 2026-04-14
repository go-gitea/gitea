// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

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
	dayStart := repo_model.NewContributorDayStart(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	stats := []*repo_model.ContributorDaily{
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
			UserID:      0,
			Email:       "bob@example.com",
			AuthorName:  "Bob",
			Commits:     5,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
		{
			RepoID:      repoID,
			DayStart:    repo_model.NewContributorDayStart(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)),
			UserID:      1,
			Email:       "alice@example.com",
			AuthorName:  "Alice",
			Commits:     2,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
	}
	_, err := db.GetEngine(t.Context()).Insert(stats)
	assert.NoError(t, err)

	contributors, total, err := repo_model.GetRepoTopContributors(t.Context(), repoID, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	if assert.Len(t, contributors, 2) {
		assert.Equal(t, "bob@example.com", contributors[0].Email)
		assert.Equal(t, int64(5), contributors[0].Commits)
		assert.Equal(t, "alice@example.com", contributors[1].Email)
		assert.Equal(t, int64(5), contributors[1].Commits)
	}
}

func TestIterateRepoIDsWithoutContributorDaily(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repoWithStats := int64(1)
	_, err := db.GetEngine(t.Context()).Insert(&repo_model.ContributorDaily{
		RepoID:      repoWithStats,
		DayStart:    repo_model.NewContributorDayStart(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		UserID:      1,
		Email:       "alice@example.com",
		AuthorName:  "Alice",
		Commits:     1,
		UpdatedUnix: timeutil.TimeStampNow(),
	})
	assert.NoError(t, err)

	collected := make(map[int64]struct{})
	iterateErr := repo_model.IterateRepoIDsWithoutContributorDaily(t.Context(), 1, func(repoIDs []int64) error {
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
	stamp := repo_model.NewContributorDayStart(time.Date(2024, 1, 2, 1, 0, 0, 0, time.FixedZone("UTC+8", 8*3600)))
	expected := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	assert.Equal(t, expected, stamp.UnixMilli())
}

func TestGetRepoTopContributorsLimit(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repoID := int64(2)
	dayStart := repo_model.NewContributorDayStart(time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC))
	_, err := db.GetEngine(t.Context()).Insert([]*repo_model.ContributorDaily{
		{
			RepoID:      repoID,
			DayStart:    dayStart,
			UserID:      0,
			Email:       "alpha@example.com",
			AuthorName:  "Alpha",
			Commits:     10,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
		{
			RepoID:      repoID,
			DayStart:    dayStart,
			UserID:      0,
			Email:       "beta@example.com",
			AuthorName:  "Beta",
			Commits:     2,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
	})
	assert.NoError(t, err)

	contributors, total, err := repo_model.GetRepoTopContributors(t.Context(), repoID, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	if assert.Len(t, contributors, 1) {
		assert.Equal(t, "alpha@example.com", contributors[0].Email)
		assert.Equal(t, int64(10), contributors[0].Commits)
	}
}
