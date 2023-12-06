// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestIsWatching(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	assert.True(t, repo_model.IsWatching(db.DefaultContext, 1, 1))
	assert.True(t, repo_model.IsWatching(db.DefaultContext, 4, 1))
	assert.True(t, repo_model.IsWatching(db.DefaultContext, 11, 1))

	assert.False(t, repo_model.IsWatching(db.DefaultContext, 1, 5))
	assert.False(t, repo_model.IsWatching(db.DefaultContext, 8, 1))
	assert.False(t, repo_model.IsWatching(db.DefaultContext, unittest.NonexistentID, unittest.NonexistentID))
}

func TestGetWatchers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	watches, err := repo_model.GetWatchers(db.DefaultContext, repo.ID)
	assert.NoError(t, err)
	// One watchers are inactive, thus minus 1
	assert.Len(t, watches, repo.NumWatches-1)
	for _, watch := range watches {
		assert.EqualValues(t, repo.ID, watch.RepoID)
	}

	watches, err = repo_model.GetWatchers(db.DefaultContext, unittest.NonexistentID)
	assert.NoError(t, err)
	assert.Len(t, watches, 0)
}

func TestRepository_GetWatchers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	watchers, err := repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, repo.NumWatches)
	for _, watcher := range watchers {
		unittest.AssertExistsAndLoadBean(t, &repo_model.Watch{UserID: watcher.ID, RepoID: repo.ID})
	}

	repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 9})
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, 0)
}

func TestWatchIfAuto(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	watchers, err := repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, repo.NumWatches)

	setting.Service.AutoWatchOnChanges = false

	prevCount := repo.NumWatches

	// Must not add watch
	assert.NoError(t, repo_model.WatchIfAuto(db.DefaultContext, 8, 1, true))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should not add watch
	assert.NoError(t, repo_model.WatchIfAuto(db.DefaultContext, 10, 1, true))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	setting.Service.AutoWatchOnChanges = true

	// Must not add watch
	assert.NoError(t, repo_model.WatchIfAuto(db.DefaultContext, 8, 1, true))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should not add watch
	assert.NoError(t, repo_model.WatchIfAuto(db.DefaultContext, 12, 1, false))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should add watch
	assert.NoError(t, repo_model.WatchIfAuto(db.DefaultContext, 12, 1, true))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount+1)

	// Should remove watch, inhibit from adding auto
	assert.NoError(t, repo_model.WatchRepo(db.DefaultContext, 12, 1, false))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Must not add watch
	assert.NoError(t, repo_model.WatchIfAuto(db.DefaultContext, 12, 1, true))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)
}

func TestWatchRepoMode(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	unittest.AssertCount(t, &repo_model.Watch{UserID: 12, RepoID: 1}, 0)

	assert.NoError(t, repo_model.WatchRepoMode(db.DefaultContext, 12, 1, repo_model.WatchModeAuto))
	unittest.AssertCount(t, &repo_model.Watch{UserID: 12, RepoID: 1}, 1)
	unittest.AssertCount(t, &repo_model.Watch{UserID: 12, RepoID: 1, Mode: repo_model.WatchModeAuto}, 1)

	assert.NoError(t, repo_model.WatchRepoMode(db.DefaultContext, 12, 1, repo_model.WatchModeNormal))
	unittest.AssertCount(t, &repo_model.Watch{UserID: 12, RepoID: 1}, 1)
	unittest.AssertCount(t, &repo_model.Watch{UserID: 12, RepoID: 1, Mode: repo_model.WatchModeNormal}, 1)

	assert.NoError(t, repo_model.WatchRepoMode(db.DefaultContext, 12, 1, repo_model.WatchModeDont))
	unittest.AssertCount(t, &repo_model.Watch{UserID: 12, RepoID: 1}, 1)
	unittest.AssertCount(t, &repo_model.Watch{UserID: 12, RepoID: 1, Mode: repo_model.WatchModeDont}, 1)

	assert.NoError(t, repo_model.WatchRepoMode(db.DefaultContext, 12, 1, repo_model.WatchModeNone))
	unittest.AssertCount(t, &repo_model.Watch{UserID: 12, RepoID: 1}, 0)
}

func checkRepoWatchersEvent(t *testing.T, event repo_model.WatchEventType, isWatching bool) {
	watchers, err := repo_model.GetRepoWatchersEventIDs(db.DefaultContext, 1, event)
	assert.NoError(t, err)

	if isWatching {
		assert.Len(t, watchers, 1)
		assert.Equal(t, int64(12), watchers[0])
	} else {
		assert.Len(t, watchers, 0)
	}
}

func TestWatchRepoCustom(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	unittest.AssertCount(t, &repo_model.Watch{UserID: 12, RepoID: 1}, 0)

	// Make sure nobody is watching this repo
	watchers, err := repo_model.GetRepoWatchersIDs(db.DefaultContext, 1)
	assert.NoError(t, err)
	for _, watcher := range watchers {
		assert.NoError(t, repo_model.WatchRepoMode(db.DefaultContext, watcher, 1, repo_model.WatchModeNone))
	}

	assert.NoError(t, repo_model.WatchRepoCustom(db.DefaultContext, 12, 1, api.RepoCustomWatchOptions{Issues: true}))
	checkRepoWatchersEvent(t, repo_model.WatchEventTypeIssue, true)
	checkRepoWatchersEvent(t, repo_model.WatchEventTypePullRequest, false)
	checkRepoWatchersEvent(t, repo_model.WatchEventTypeRelease, false)

	assert.NoError(t, repo_model.WatchRepoCustom(db.DefaultContext, 12, 1, api.RepoCustomWatchOptions{PullRequests: true}))
	checkRepoWatchersEvent(t, repo_model.WatchEventTypeIssue, false)
	checkRepoWatchersEvent(t, repo_model.WatchEventTypePullRequest, true)
	checkRepoWatchersEvent(t, repo_model.WatchEventTypeRelease, false)

	assert.NoError(t, repo_model.WatchRepoCustom(db.DefaultContext, 12, 1, api.RepoCustomWatchOptions{Releases: true}))
	checkRepoWatchersEvent(t, repo_model.WatchEventTypeIssue, false)
	checkRepoWatchersEvent(t, repo_model.WatchEventTypePullRequest, false)
	checkRepoWatchersEvent(t, repo_model.WatchEventTypeRelease, true)
}
