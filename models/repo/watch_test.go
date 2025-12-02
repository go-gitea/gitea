// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
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
	watches, err := repo_model.GetWatchers(t.Context(), repo.ID)
	assert.NoError(t, err)
	// One watchers are inactive, thus minus 1
	assert.Len(t, watches, repo.NumWatches-1)
	for _, watch := range watches {
		assert.Equal(t, repo.ID, watch.RepoID)
	}

	watches, err = repo_model.GetWatchers(t.Context(), unittest.NonexistentID)
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
