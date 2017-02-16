// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsWatching(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	assert.True(t, IsWatching(1, 1))
	assert.True(t, IsWatching(4, 1))

	assert.False(t, IsWatching(1, 5))
	assert.False(t, IsWatching(NonexistentID, NonexistentID))
}

func TestWatchRepo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	const repoID = 3
	const userID = 2

	assert.NoError(t, WatchRepo(userID, repoID, true))
	AssertExistsAndLoadBean(t, &Watch{RepoID: repoID, UserID: userID})
	CheckConsistencyFor(t, &Repository{ID: repoID})

	assert.NoError(t, WatchRepo(userID, repoID, false))
	AssertNotExistsBean(t, &Watch{RepoID: repoID, UserID: userID})
	CheckConsistencyFor(t, &Repository{ID: repoID})
}

func TestGetWatchers(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	watches, err := GetWatchers(repo.ID)
	assert.NoError(t, err)
	assert.Len(t, watches, repo.NumWatches)
	for _, watch := range watches {
		assert.EqualValues(t, repo.ID, watch.RepoID)
	}

	watches, err = GetWatchers(NonexistentID)
	assert.NoError(t, err)
	assert.Len(t, watches, 0)
}

func TestRepository_GetWatchers(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	watchers, err := repo.GetWatchers(1)
	assert.NoError(t, err)
	assert.Len(t, watchers, repo.NumWatches)
	for _, watcher := range watchers {
		AssertExistsAndLoadBean(t, &Watch{UserID: watcher.ID, RepoID: repo.ID})
	}

	repo = AssertExistsAndLoadBean(t, &Repository{ID: 10}).(*Repository)
	watchers, err = repo.GetWatchers(1)
	assert.NoError(t, err)
	assert.Len(t, watchers, 0)
}

func TestNotifyWatchers(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	action := &Action{
		ActUserID: 8,
		RepoID:    1,
		OpType:    ActionStarRepo,
	}
	assert.NoError(t, NotifyWatchers(action))

	AssertExistsAndLoadBean(t, &Action{
		ActUserID: action.ActUserID,
		UserID:    1,
		RepoID:    action.RepoID,
		OpType:    action.OpType,
	})
	AssertExistsAndLoadBean(t, &Action{
		ActUserID: action.ActUserID,
		UserID:    4,
		RepoID:    action.RepoID,
		OpType:    action.OpType,
	})
	AssertExistsAndLoadBean(t, &Action{
		ActUserID: action.ActUserID,
		UserID:    8,
		RepoID:    action.RepoID,
		OpType:    action.OpType,
	})
}
