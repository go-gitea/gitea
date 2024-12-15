// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"testing"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestGetFeeds(t *testing.T) {
	// test with an individual user
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	actions, count, err := GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedUser:   user,
		Actor:           user,
		IncludePrivate:  true,
		OnlyPerformedBy: false,
		IncludeDeleted:  true,
	})
	assert.NoError(t, err)
	if assert.Len(t, actions, 1) {
		assert.EqualValues(t, 1, actions[0].ID)
		assert.EqualValues(t, user.ID, actions[0].UserID)
	}
	assert.Equal(t, int64(1), count)

	actions, count, err = GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedUser:   user,
		Actor:           user,
		IncludePrivate:  false,
		OnlyPerformedBy: false,
	})
	assert.NoError(t, err)
	assert.Empty(t, actions)
	assert.Equal(t, int64(0), count)
}

func TestGetFeedsForRepos(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	privRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	pubRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 8})

	// private repo & no login
	actions, count, err := GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedRepo:  privRepo,
		IncludePrivate: true,
	})
	assert.NoError(t, err)
	assert.Empty(t, actions)
	assert.Equal(t, int64(0), count)

	// public repo & no login
	actions, count, err = GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedRepo:  pubRepo,
		IncludePrivate: true,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 1)
	assert.Equal(t, int64(1), count)

	// private repo and login
	actions, count, err = GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedRepo:  privRepo,
		IncludePrivate: true,
		Actor:          user,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 1)
	assert.Equal(t, int64(1), count)

	// public repo & login
	actions, count, err = GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedRepo:  pubRepo,
		IncludePrivate: true,
		Actor:          user,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 1)
	assert.Equal(t, int64(1), count)
}

func TestGetFeeds2(t *testing.T) {
	// test with an organization user
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	actions, count, err := GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedUser:   org,
		Actor:           user,
		IncludePrivate:  true,
		OnlyPerformedBy: false,
		IncludeDeleted:  true,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 1)
	if assert.Len(t, actions, 1) {
		assert.EqualValues(t, 2, actions[0].ID)
		assert.EqualValues(t, org.ID, actions[0].UserID)
	}
	assert.Equal(t, int64(1), count)

	actions, count, err = GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedUser:   org,
		Actor:           user,
		IncludePrivate:  false,
		OnlyPerformedBy: false,
		IncludeDeleted:  true,
	})
	assert.NoError(t, err)
	assert.Empty(t, actions)
	assert.Equal(t, int64(0), count)
}

func TestGetFeedsCorrupted(t *testing.T) {
	// Now we will not check for corrupted data in the feeds
	// users should run doctor to fix their data
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	unittest.AssertExistsAndLoadBean(t, &activities_model.Action{
		ID:     8,
		RepoID: 1700,
	})

	actions, count, err := GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedUser:  user,
		Actor:          user,
		IncludePrivate: true,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 1)
	assert.Equal(t, int64(1), count)
}

func TestRepoActions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	_ = db.TruncateBeans(db.DefaultContext, &activities_model.Action{})
	for i := 0; i < 3; i++ {
		_ = db.Insert(db.DefaultContext, &activities_model.Action{
			UserID:    2 + int64(i),
			ActUserID: 2,
			RepoID:    repo.ID,
			OpType:    activities_model.ActionCommentIssue,
		})
	}
	count, _ := db.Count[activities_model.Action](db.DefaultContext, &db.ListOptions{})
	assert.EqualValues(t, 3, count)
	actions, _, err := GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedRepo: repo,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 1)
}
