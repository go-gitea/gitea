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
)

func TestLinkedRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	testCases := []struct {
		name             string
		attachID         int64
		expectedRepo     *repo_model.Repository
		expectedUnitType unit.Type
	}{
		{"LinkedIssue", 1, &repo_model.Repository{ID: 1}, unit.TypeIssues},
		{"LinkedComment", 3, &repo_model.Repository{ID: 1}, unit.TypePullRequests},
		{"LinkedRelease", 9, &repo_model.Repository{ID: 1}, unit.TypeReleases},
		{"Notlinked", 10, nil, -1},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			attach, err := repo_model.GetAttachmentByID(t.Context(), tc.attachID)
			assert.NoError(t, err)
			repo, unitType, err := LinkedRepository(t.Context(), attach)
			assert.NoError(t, err)
			if tc.expectedRepo != nil {
				assert.Equal(t, tc.expectedRepo.ID, repo.ID)
			}
			assert.Equal(t, tc.expectedUnitType, unitType)
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
