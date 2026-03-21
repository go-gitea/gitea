// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"

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
	err = updateRepository(t.Context(), repo, true)
	assert.NoError(t, err)

	// Check visibility of action has become private
	act := activities_model.Action{}
	_, err = db.GetEngine(t.Context()).ID(3).Get(&act)

	assert.NoError(t, err)
	assert.True(t, act.IsPrivate)
}

func TestRepository_HasWiki(t *testing.T) {
	unittest.PrepareTestEnv(t)
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.True(t, HasWiki(t.Context(), repo1))

	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	assert.False(t, HasWiki(t.Context(), repo2))
}

func TestMakeRepoPrivateClearsWatches(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repo.IsPrivate = false

	watchers, err := repo_model.GetRepoWatchersIDs(t.Context(), repo.ID)
	require.NoError(t, err)
	require.NotEmpty(t, watchers)

	assert.NoError(t, MakeRepoPrivate(t.Context(), repo))

	watchers, err = repo_model.GetRepoWatchersIDs(t.Context(), repo.ID)
	assert.NoError(t, err)
	assert.Empty(t, watchers)

	updatedRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
	assert.True(t, updatedRepo.IsPrivate)
	assert.Zero(t, updatedRepo.NumWatches)
}
