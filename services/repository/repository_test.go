// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	activities_model "gitea.dev/models/activities"
	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachLinkedTypeAndRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	testCases := []struct {
		name             string
		attachID         int64
		expectedUnitType unit.Type
		expectedRepoID   int64
	}{
		{"LinkedIssue", 1, unit.TypeIssues, 1},
		{"LinkedComment", 3, unit.TypePullRequests, 1},
		{"LinkedRelease", 9, unit.TypeReleases, 1},
		{"Notlinked", 10, unit.TypeInvalid, 0},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			attach, err := repo_model.GetAttachmentByID(t.Context(), tc.attachID)
			assert.NoError(t, err)
			unitType, repoID, err := GetAttachmentLinkedTypeAndRepoID(t.Context(), attach)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedUnitType, unitType)
			assert.Equal(t, tc.expectedRepoID, repoID)
		})
	}
}

func TestUpdateRepositoryVisibilityChanged(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Get sample repo and change visibility
	repo, err := repo_model.GetRepositoryByID(t.Context(), 9)
	assert.NoError(t, err)
	repo.IsPrivate = true

	// Update it
	err = UpdateRepository(t.Context(), repo, UpdateRepositoryOptions{VisibilityChanged: true})
	assert.NoError(t, err)

	// Check visibility of action has become private
	act := activities_model.Action{}
	_, err = db.GetEngine(t.Context()).ID(3).Get(&act)

	assert.NoError(t, err)
	assert.True(t, act.IsPrivate)
}

func TestSetArchiveRepoStateDisablesPages(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	require.NoError(t, pages_model.InsertSite(t.Context(), &pages_model.Site{
		RepoID: 1, Enabled: true, SourceType: pages_model.PublishSourceArtifact, PagesUpdatedUnixNano: 1,
	}))
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	require.NoError(t, SetArchiveRepoState(t.Context(), repo, true))
	archived := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.True(t, archived.IsArchived)
	site, has, err := pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, has)
	assert.False(t, site.Enabled)
	assert.Greater(t, site.PagesUpdatedUnixNano, int64(1))

	require.NoError(t, SetArchiveRepoState(t.Context(), repo, false))
	unarchived := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.False(t, unarchived.IsArchived)
}

func TestRepository_HasWiki(t *testing.T) {
	unittest.PrepareTestEnv(t)
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.True(t, HasWiki(t.Context(), repo1))

	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	assert.False(t, HasWiki(t.Context(), repo2))
}

func TestMakeRepoPrivateClearsWatches(t *testing.T) {
	for _, name := range []string{"MakeRepoPrivate", "UpdateRepository"} {
		t.Run(name, func(t *testing.T) {
			assert.NoError(t, unittest.PrepareTestDatabase())

			repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
			assert.False(t, repo.IsPrivate)

			watchers, err := repo_model.GetRepoWatchers(t.Context(), repo.ID, db.ListOptions{Page: 1})
			require.NoError(t, err)
			require.NotEmpty(t, watchers)

			if name == "MakeRepoPrivate" {
				assert.NoError(t, MakeRepoPrivate(t.Context(), repo, true))
			} else {
				repo.IsPrivate = true
				require.NoError(t, UpdateRepository(t.Context(), repo, UpdateRepositoryOptions{VisibilityChanged: true}))
			}

			watchers, err = repo_model.GetRepoWatchers(t.Context(), repo.ID, db.ListOptions{Page: 1})
			assert.NoError(t, err)
			assert.Empty(t, watchers)

			updatedRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
			if name == "MakeRepoPrivate" {
				assert.True(t, updatedRepo.IsPrivate)
			}
			assert.Zero(t, updatedRepo.NumWatches)
		})
	}
}

func TestMakeRepoPrivateUpdatesForks(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	fork := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})
	assert.False(t, repo.IsPrivate)
	assert.False(t, fork.IsPrivate)

	assert.NoError(t, MakeRepoPrivate(t.Context(), repo, true))

	updatedRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
	updatedFork := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: fork.ID})
	assert.True(t, updatedRepo.IsPrivate)
	assert.True(t, updatedFork.IsPrivate)
}
