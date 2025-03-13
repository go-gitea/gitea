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
	assert.Empty(t, watches)
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
	assert.Empty(t, watchers)
}

func TestWatchIfAuto(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user12 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 12})

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
	assert.NoError(t, repo_model.WatchRepo(db.DefaultContext, user12, repo, false))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Must not add watch
	assert.NoError(t, repo_model.WatchIfAuto(db.DefaultContext, 12, 1, true))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	assert.NoError(t, err)
	assert.Len(t, watchers, prevCount)
}
