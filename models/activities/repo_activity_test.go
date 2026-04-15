// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	contribution_model "code.gitea.io/gitea/models/repo/contribution"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestGetCodeActivityStats(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	dayStart := contribution_model.NewContributorDayStart(time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC))
	_, err := db.GetEngine(t.Context()).Insert([]*contribution_model.ContributorDaily{
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
