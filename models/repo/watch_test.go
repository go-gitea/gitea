// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsWatching(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	assert.True(t, repo_model.IsWatching(t.Context(), 1, 1))
	assert.True(t, repo_model.IsWatching(t.Context(), 4, 1))
	assert.True(t, repo_model.IsWatching(t.Context(), 11, 1))

	assert.False(t, repo_model.IsWatching(t.Context(), 1, 5))
	assert.False(t, repo_model.IsWatching(t.Context(), 8, 1))
	assert.False(t, repo_model.IsWatching(t.Context(), unittest.NonexistentID, unittest.NonexistentID))
}

func TestGetWatchers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	watches, err := repo_model.GetRepoWatches(t.Context(), repo.ID)
	assert.NoError(t, err)
	// One watchers are inactive, thus minus 1
	assert.Len(t, watches, repo.NumWatches-1)
	for _, watch := range watches {
		assert.Equal(t, repo.ID, watch.RepoID)
	}

	watches, err = repo_model.GetRepoWatches(t.Context(), unittest.NonexistentID)
	assert.NoError(t, err)
	assert.Empty(t, watches)
}

func TestRepository_GetWatchers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	watchers, err := repo_model.GetRepoWatchers(t.Context(), repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, repo.NumWatches)
	for _, watcher := range watchers {
		unittest.AssertExistsAndLoadBean(t, &repo_model.Watch{UserID: watcher.ID, RepoID: repo.ID})
	}

	repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 9})
	watchers, err = repo_model.GetRepoWatchers(t.Context(), repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Empty(t, watchers)
}

func TestWatchIfAuto(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user12 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 12})

	watchers, err := repo_model.GetRepoWatchers(t.Context(), repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, repo.NumWatches)

	setting.Service.AutoWatchOnChanges = false

	prevCount := repo.NumWatches

	// Must not add watch
	assert.NoError(t, repo_model.WatchIfAuto(t.Context(), 8, 1, true))
	watchers, err = repo_model.GetRepoWatchers(t.Context(), repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should not add watch
	assert.NoError(t, repo_model.WatchIfAuto(t.Context(), 10, 1, true))
	watchers, err = repo_model.GetRepoWatchers(t.Context(), repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	setting.Service.AutoWatchOnChanges = true

	// Must not add watch
	assert.NoError(t, repo_model.WatchIfAuto(t.Context(), 8, 1, true))
	watchers, err = repo_model.GetRepoWatchers(t.Context(), repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should not add watch
	assert.NoError(t, repo_model.WatchIfAuto(t.Context(), 12, 1, false))
	watchers, err = repo_model.GetRepoWatchers(t.Context(), repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should add watch
	assert.NoError(t, repo_model.WatchIfAuto(t.Context(), 12, 1, true))
	watchers, err = repo_model.GetRepoWatchers(t.Context(), repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount+1)

	// Should remove watch, inhibit from adding auto
	assert.NoError(t, repo_model.WatchRepo(t.Context(), user12, repo, false))
	watchers, err = repo_model.GetRepoWatchers(t.Context(), repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Must not add watch
	assert.NoError(t, repo_model.WatchIfAuto(t.Context(), 12, 1, true))
	watchers, err = repo_model.GetRepoWatchers(t.Context(), repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)
}

func TestClearRepoWatches(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const repoID int64 = 1
	watchers, err := repo_model.GetRepoWatchers(t.Context(), repoID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	require.NotEmpty(t, watchers)

	assert.NoError(t, repo_model.ClearRepoWatches(t.Context(), repoID))

	watchers, err = repo_model.GetRepoWatchers(t.Context(), repoID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Empty(t, watchers)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
	assert.Zero(t, repo.NumWatches)
}

func TestWatchOptions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 5})
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	assert.NoError(t, repo_model.WatchRepo(t.Context(), user5, repo, true))
	watch, err := repo_model.GetWatch(t.Context(), user5.ID, repo.ID)
	assert.NoError(t, err)
	assert.True(t, watch.PullRequests)
	assert.True(t, watch.Issues)
	assert.True(t, watch.Releases)

	assert.NoError(t, repo_model.WatchRepoOptions(t.Context(), user5, repo, repo_model.WatchOptions{
		PullRequests: true,
		Issues:       false,
		Releases:     true,
	}))
	watch, err = repo_model.GetWatch(t.Context(), user5.ID, repo.ID)
	assert.NoError(t, err)
	assert.True(t, watch.PullRequests)
	assert.False(t, watch.Issues)
	assert.True(t, watch.Releases)
}
