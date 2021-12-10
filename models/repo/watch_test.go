// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

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
	assert.False(t, IsWatching(unittest.NonexistentID, unittest.NonexistentID))
}

func TestGetWatchers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	watches, err := GetWatchers(db.DefaultContext, repo.ID)
	assert.NoError(t, err)
	// One watchers are inactive, thus minus 1
	assert.Len(t, watches, repo.NumWatches-1)
	for _, watch := range watches {
		assert.EqualValues(t, repo.ID, watch.RepoID)
	}

	watches, err = GetWatchers(db.DefaultContext, unittest.NonexistentID)
	assert.NoError(t, err)
	assert.Len(t, watches, 0)
}

func TestRepository_GetWatchers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	watchers, err := GetRepoWatchers(repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, repo.NumWatches)
	for _, watcher := range watchers {
		unittest.AssertExistsAndLoadBean(t, &Watch{UserID: watcher.ID, RepoID: repo.ID})
	}

	repo = unittest.AssertExistsAndLoadBean(t, &Repository{ID: 9}).(*Repository)
	watchers, err = GetRepoWatchers(repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, 0)
}

func TestWatchIfAuto(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	watchers, err := GetRepoWatchers(repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, repo.NumWatches)

	setting.Service.AutoWatchOnChanges = false

	prevCount := repo.NumWatches

	// Must not add watch
	assert.NoError(t, WatchIfAuto(8, 1, true))
	watchers, err = GetRepoWatchers(repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should not add watch
	assert.NoError(t, WatchIfAuto(10, 1, true))
	watchers, err = GetRepoWatchers(repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	setting.Service.AutoWatchOnChanges = true

	// Must not add watch
	assert.NoError(t, WatchIfAuto(8, 1, true))
	watchers, err = GetRepoWatchers(repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should not add watch
	assert.NoError(t, WatchIfAuto(12, 1, false))
	watchers, err = GetRepoWatchers(repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should add watch
	assert.NoError(t, WatchIfAuto(12, 1, true))
	watchers, err = GetRepoWatchers(repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount+1)

	// Should remove watch, inhibit from adding auto
	assert.NoError(t, WatchRepo(12, 1, false))
	watchers, err = GetRepoWatchers(repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Must not add watch
	assert.NoError(t, WatchIfAuto(12, 1, true))
	watchers, err = GetRepoWatchers(repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)
}

func TestWatchRepoMode(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	unittest.AssertCount(t, &Watch{UserID: 12, RepoID: 1}, 0)

	assert.NoError(t, WatchRepoMode(12, 1, WatchModeAuto))
	unittest.AssertCount(t, &Watch{UserID: 12, RepoID: 1}, 1)
	unittest.AssertCount(t, &Watch{UserID: 12, RepoID: 1, Mode: WatchModeAuto}, 1)

	assert.NoError(t, WatchRepoMode(12, 1, WatchModeNormal))
	unittest.AssertCount(t, &Watch{UserID: 12, RepoID: 1}, 1)
	unittest.AssertCount(t, &Watch{UserID: 12, RepoID: 1, Mode: WatchModeNormal}, 1)

	assert.NoError(t, WatchRepoMode(12, 1, WatchModeDont))
	unittest.AssertCount(t, &Watch{UserID: 12, RepoID: 1}, 1)
	unittest.AssertCount(t, &Watch{UserID: 12, RepoID: 1, Mode: WatchModeDont}, 1)

	assert.NoError(t, WatchRepoMode(12, 1, WatchModeNone))
	unittest.AssertCount(t, &Watch{UserID: 12, RepoID: 1}, 0)
}
