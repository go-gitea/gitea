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

	assert.True(t, repo_model.IsWatchingRepo(t.Context(), 1, 1))
	assert.True(t, repo_model.IsWatchingRepo(t.Context(), 4, 1))
	assert.True(t, repo_model.IsWatchingRepo(t.Context(), 11, 1))

	assert.False(t, repo_model.IsWatchingRepo(t.Context(), 1, 5))
	assert.False(t, repo_model.IsWatchingRepo(t.Context(), 8, 1))
	assert.False(t, repo_model.IsWatchingRepo(t.Context(), unittest.NonexistentID, unittest.NonexistentID))
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
	assert.NoError(t, repo_model.WatchRepoAuto(t.Context(), user12, repo, false))
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

	// repo 1 is watched by users 1, 4, 9 and 11, all with every event enabled
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	assert.NoError(t, repo_model.WatchRepoWithOptions(t.Context(), user, repo, repo_model.WatchOptions{Mode: repo_model.WatchModeNormal, WatchPullRequests: true}))

	for watchType, expected := range map[repo_model.WatchType][]int64{
		repo_model.WatchPullRequests: {1, 4, 9, 11},
		repo_model.WatchIssues:       {4, 9, 11},
		repo_model.WatchReleases:     {4, 9, 11},
	} {
		ids, err := repo_model.GetRepoWatchersIDs(t.Context(), repo.ID, watchType)
		assert.NoError(t, err)
		assert.ElementsMatch(t, expected, ids, watchType)
	}

	// the options of one user must not show up for another
	watches, err := repo_model.GetUserWatches(t.Context(), 4, []int64{repo.ID})
	assert.NoError(t, err)
	assert.True(t, watches[repo.ID].IncludeIssues)

	// watching again resets a custom selection
	assert.NoError(t, repo_model.WatchRepoAuto(t.Context(), user, repo, false))
	assert.NoError(t, repo_model.WatchRepoAuto(t.Context(), user, repo, true))
	watch, err := repo_model.GetWatch(t.Context(), user.ID, repo.ID)
	assert.NoError(t, err)
	assert.True(t, watch.IsWatchingAll())
}

func TestWatchSelectedMode(t *testing.T) {
	// a user without a watch row gets the dummy record, whose flags are the column defaults
	assert.Equal(t, "participate", (&repo_model.Watch{Mode: repo_model.WatchModeNone, IncludePullRequests: true, IncludeIssues: true, IncludeReleases: true}).SelectedMode())
	assert.Equal(t, "participate", (&repo_model.Watch{Mode: repo_model.WatchModeNormal}).SelectedMode())
	assert.Equal(t, "ignore", (&repo_model.Watch{Mode: repo_model.WatchModeDont}).SelectedMode())
	assert.Equal(t, "custom", (&repo_model.Watch{Mode: repo_model.WatchModeNormal, IncludeIssues: true}).SelectedMode())
	assert.Equal(t, "all", (&repo_model.Watch{Mode: repo_model.WatchModeAuto, IncludePullRequests: true, IncludeIssues: true, IncludeReleases: true}).SelectedMode())
}
