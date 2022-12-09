// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestRepository_GetCollaborators(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(repoID int64) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
		collaborators, err := repo_model.GetCollaborators(db.DefaultContext, repo.ID, db.ListOptions{})
		assert.NoError(t, err)
		expectedLen, err := db.GetEngine(db.DefaultContext).Count(&repo_model.Collaboration{RepoID: repoID})
		assert.NoError(t, err)
		assert.Len(t, collaborators, int(expectedLen))
		for _, collaborator := range collaborators {
			assert.EqualValues(t, collaborator.User.ID, collaborator.Collaboration.UserID)
			assert.EqualValues(t, repoID, collaborator.Collaboration.RepoID)
		}
	}
	test(1)
	test(2)
	test(3)
	test(4)
}

func TestRepository_IsCollaborator(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(repoID, userID int64, expected bool) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
		actual, err := repo_model.IsCollaborator(db.DefaultContext, repo.ID, userID)
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	}
	test(3, 2, true)
	test(3, unittest.NonexistentID, false)
	test(4, 2, false)
	test(4, 4, true)
}

func TestRepository_ChangeCollaborationAccessMode(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	assert.NoError(t, repo_model.ChangeCollaborationAccessMode(repo, 4, perm.AccessModeAdmin))

	collaboration := unittest.AssertExistsAndLoadBean(t, &repo_model.Collaboration{RepoID: repo.ID, UserID: 4})
	assert.EqualValues(t, perm.AccessModeAdmin, collaboration.Mode)

	access := unittest.AssertExistsAndLoadBean(t, &access_model.Access{UserID: 4, RepoID: repo.ID})
	assert.EqualValues(t, perm.AccessModeAdmin, access.Mode)

	assert.NoError(t, repo_model.ChangeCollaborationAccessMode(repo, 4, perm.AccessModeAdmin))

	assert.NoError(t, repo_model.ChangeCollaborationAccessMode(repo, unittest.NonexistentID, perm.AccessModeAdmin))

	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: repo.ID})
}
