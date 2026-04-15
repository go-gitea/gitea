// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestGetContributorActivity(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	firstDay := repo_model.NewContributorDayStart(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	secondDay := repo_model.NewContributorDayStart(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	_, err := db.GetEngine(t.Context()).Insert([]*repo_model.ContributorDaily{
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

	rows, err := getContributorActivity(t.Context(), repo, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 10)
	assert.NoError(t, err)
	assert.Len(t, rows, 2)

	var alpha *contributorActivityRow
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

func TestGetCodeActivityStats(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	dayStart := repo_model.NewContributorDayStart(time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC))
	_, err := db.GetEngine(t.Context()).Insert([]*repo_model.ContributorDaily{
		{
			RepoID:      repo.ID,
			DayStart:    dayStart,
			UserID:      1,
			Email:       "alpha@example.com",
			AuthorName:  "Alpha",
			Additions:   5,
			Deletions:   2,
			Commits:     4,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
		{
			RepoID:      repo.ID,
			DayStart:    dayStart,
			UserID:      0,
			Email:       "beta@example.com",
			AuthorName:  "Beta",
			Additions:   1,
			Deletions:   1,
			Commits:     1,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
	})
	assert.NoError(t, err)

	stats, err := getCodeActivityStats(t.Context(), repo, time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC))
	assert.NoError(t, err)
	assert.Equal(t, int64(6), stats.Additions)
	assert.Equal(t, int64(3), stats.Deletions)
	assert.Equal(t, int64(5), stats.CommitCount)
	assert.Equal(t, int64(2), stats.AuthorCount)
}
