// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestIsWatching(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	assert.True(t, IsWatching(1, 1))
	assert.True(t, IsWatching(4, 1))
	assert.True(t, IsWatching(11, 1))

	assert.False(t, IsWatching(1, 5))
	assert.False(t, IsWatching(8, 1))
	assert.False(t, IsWatching(db.NonexistentID, db.NonexistentID))
}

func TestWatchRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	const repoID = 3
	const userID = 2

	assert.NoError(t, WatchRepo(userID, repoID, true))
	db.AssertExistsAndLoadBean(t, &Watch{RepoID: repoID, UserID: userID})
	CheckConsistencyFor(t, &Repository{ID: repoID})

	assert.NoError(t, WatchRepo(userID, repoID, false))
	db.AssertNotExistsBean(t, &Watch{RepoID: repoID, UserID: userID})
	CheckConsistencyFor(t, &Repository{ID: repoID})
}

func TestGetWatchers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := db.AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	watches, err := GetWatchers(repo.ID)
	assert.NoError(t, err)
	// One watchers are inactive, thus minus 1
	assert.Len(t, watches, repo.NumWatches-1)
	for _, watch := range watches {
		assert.EqualValues(t, repo.ID, watch.RepoID)
	}

	watches, err = GetWatchers(db.NonexistentID)
	assert.NoError(t, err)
	assert.Len(t, watches, 0)
}

func TestRepository_GetWatchers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := db.AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	watchers, err := repo.GetWatchers(db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, repo.NumWatches)
	for _, watcher := range watchers {
		db.AssertExistsAndLoadBean(t, &Watch{UserID: watcher.ID, RepoID: repo.ID})
	}

	repo = db.AssertExistsAndLoadBean(t, &Repository{ID: 9}).(*Repository)
	watchers, err = repo.GetWatchers(db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, 0)
}

func TestNotifyWatchers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	action := &Action{
		ActUserID: 8,
		RepoID:    1,
		OpType:    ActionStarRepo,
	}
	assert.NoError(t, NotifyWatchers(action))

	// One watchers are inactive, thus action is only created for user 8, 1, 4, 11
	db.AssertExistsAndLoadBean(t, &Action{
		ActUserID: action.ActUserID,
		UserID:    8,
		RepoID:    action.RepoID,
		OpType:    action.OpType,
	})
	db.AssertExistsAndLoadBean(t, &Action{
		ActUserID: action.ActUserID,
		UserID:    1,
		RepoID:    action.RepoID,
		OpType:    action.OpType,
	})
	db.AssertExistsAndLoadBean(t, &Action{
		ActUserID: action.ActUserID,
		UserID:    4,
		RepoID:    action.RepoID,
		OpType:    action.OpType,
	})
	db.AssertExistsAndLoadBean(t, &Action{
		ActUserID: action.ActUserID,
		UserID:    11,
		RepoID:    action.RepoID,
		OpType:    action.OpType,
	})
}

func TestWatchIfAuto(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := db.AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	watchers, err := repo.GetWatchers(db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, repo.NumWatches)

	setting.Service.AutoWatchOnChanges = false

	prevCount := repo.NumWatches

	// Must not add watch
	assert.NoError(t, WatchIfAuto(8, 1, true))
	watchers, err = repo.GetWatchers(db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should not add watch
	assert.NoError(t, WatchIfAuto(10, 1, true))
	watchers, err = repo.GetWatchers(db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	setting.Service.AutoWatchOnChanges = true

	// Must not add watch
	assert.NoError(t, WatchIfAuto(8, 1, true))
	watchers, err = repo.GetWatchers(db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should not add watch
	assert.NoError(t, WatchIfAuto(12, 1, false))
	watchers, err = repo.GetWatchers(db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should add watch
	assert.NoError(t, WatchIfAuto(12, 1, true))
	watchers, err = repo.GetWatchers(db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount+1)

	// Should remove watch, inhibit from adding auto
	assert.NoError(t, WatchRepo(12, 1, false))
	watchers, err = repo.GetWatchers(db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Must not add watch
	assert.NoError(t, WatchIfAuto(12, 1, true))
	watchers, err = repo.GetWatchers(db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)
}

func TestWatchRepoMode(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	db.AssertCount(t, &Watch{UserID: 12, RepoID: 1}, 0)

	assert.NoError(t, WatchRepoMode(12, 1, RepoWatchModeAuto))
	db.AssertCount(t, &Watch{UserID: 12, RepoID: 1}, 1)
	db.AssertCount(t, &Watch{UserID: 12, RepoID: 1, Mode: RepoWatchModeAuto}, 1)

	assert.NoError(t, WatchRepoMode(12, 1, RepoWatchModeNormal))
	db.AssertCount(t, &Watch{UserID: 12, RepoID: 1}, 1)
	db.AssertCount(t, &Watch{UserID: 12, RepoID: 1, Mode: RepoWatchModeNormal}, 1)

	assert.NoError(t, WatchRepoMode(12, 1, RepoWatchModeDont))
	db.AssertCount(t, &Watch{UserID: 12, RepoID: 1}, 1)
	db.AssertCount(t, &Watch{UserID: 12, RepoID: 1, Mode: RepoWatchModeDont}, 1)

	assert.NoError(t, WatchRepoMode(12, 1, RepoWatchModeNone))
	db.AssertCount(t, &Watch{UserID: 12, RepoID: 1}, 0)
}
