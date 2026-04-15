// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

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

func TestGetContributionsOverTime(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	weekStart := time.Date(2024, 4, 7, 0, 0, 0, 0, time.UTC)
	start := contribution_model.NewContributorDayStart(weekStart.Add(24 * time.Hour))
	_, err := db.GetEngine(t.Context()).Insert([]*contribution_model.ContributorDaily{
		{
			RepoID:      repo.ID,
			DayStart:    start,
			UserID:      1,
			Email:       "alpha@example.com",
			AuthorName:  "Alpha",
			Additions:   4,
			Deletions:   1,
			Commits:     2,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
		{
			RepoID:      repo.ID,
			DayStart:    start,
			UserID:      2,
			Email:       "beta@example.com",
			AuthorName:  "Beta",
			Additions:   3,
			Deletions:   2,
			Commits:     3,
			UpdatedUnix: timeutil.TimeStampNow(),
		},
	})
	assert.NoError(t, err)

	weekDatas, err := GetContributionsOverTime(
		t.Context(),
		repo,
		nil,
		nil,
		contribution_model.RepoStatCommits,
		contribution_model.RepoStatAdditions,
		contribution_model.RepoStatDeletions,
	)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), weekDatas[weekStart.UnixMilli()].Commits)
	assert.Equal(t, int64(7), weekDatas[weekStart.UnixMilli()].Additions)
	assert.Equal(t, int64(3), weekDatas[weekStart.UnixMilli()].Deletions)
}
