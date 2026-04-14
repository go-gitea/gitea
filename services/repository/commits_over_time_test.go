// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestGetCommitsOverTime(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	start := repo_model.NewContributorDayStart(time.Now().UTC())
	_, err := db.GetEngine(t.Context()).Insert([]*repo_model.ContributorDaily{
		{
			RepoID:      repo.ID,
			DayStart:    start,
			UserID:      1,
			Email:       "alpha@example.com",
			AuthorName:  "Alpha",
			Commits:     2,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
		{
			RepoID:      repo.ID,
			DayStart:    start,
			UserID:      2,
			Email:       "beta@example.com",
			AuthorName:  "Beta",
			Commits:     3,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
	})
	assert.NoError(t, err)

	weekly, err := GetCommitsOverTime(t.Context(), repo)
	assert.NoError(t, err)
	if assert.Len(t, weekly, 1) {
		assert.Equal(t, weekStartUnixMilliFromDayStart(start), weekly[0].Week)
		assert.Equal(t, int64(5), weekly[0].Commits)
	}
}
