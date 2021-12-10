// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestUpdateRepositoryVisibilityChanged(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Get sample repo and change visibility
	repo, err := repo_model.GetRepositoryByID(9)
	assert.NoError(t, err)
	repo.IsPrivate = true

	// Update it
	err = UpdateRepository(repo, true)
	assert.NoError(t, err)

	// Check visibility of action has become private
	act := Action{}
	_, err = db.GetEngine(db.DefaultContext).ID(3).Get(&act)

	assert.NoError(t, err)
	assert.True(t, act.IsPrivate)
}

func TestDoctorUserStarNum(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	assert.NoError(t, DoctorUserStarNum())
}

func TestRepoGetReviewers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// test public repo
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)

	reviewers, err := GetReviewers(repo1, 2, 2)
	assert.NoError(t, err)
	assert.Len(t, reviewers, 4)

	// test private repo
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2}).(*repo_model.Repository)
	reviewers, err = GetReviewers(repo2, 2, 2)
	assert.NoError(t, err)
	assert.Empty(t, reviewers)
}

func TestRepoGetReviewerTeams(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2}).(*repo_model.Repository)
	teams, err := GetReviewerTeams(repo2)
	assert.NoError(t, err)
	assert.Empty(t, teams)

	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3}).(*repo_model.Repository)
	teams, err = GetReviewerTeams(repo3)
	assert.NoError(t, err)
	assert.Len(t, teams, 2)
}

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
			attach, err := repo_model.GetAttachmentByID(tc.attachID)
			assert.NoError(t, err)
			repo, unitType, err := LinkedRepository(attach)
			assert.NoError(t, err)
			if tc.expectedRepo != nil {
				assert.Equal(t, tc.expectedRepo.ID, repo.ID)
			}
			assert.Equal(t, tc.expectedUnitType, unitType)
		})
	}
}
