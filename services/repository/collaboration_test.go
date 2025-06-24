// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestRepository_AddCollaborator(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(repoID, userID int64) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
		assert.NoError(t, repo.LoadOwner(db.DefaultContext))
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userID})
		assert.NoError(t, AddOrUpdateCollaborator(db.DefaultContext, repo, user, perm.AccessModeWrite))
		unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: repoID}, &user_model.User{ID: userID})
	}
	testSuccess(1, 4)
	testSuccess(1, 4)
	testSuccess(3, 4)
}

func TestRepository_DeleteCollaboration(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})

	assert.NoError(t, repo.LoadOwner(db.DefaultContext))
	assert.NoError(t, DeleteCollaboration(db.DefaultContext, repo, user))
	unittest.AssertNotExistsBean(t, &repo_model.Collaboration{RepoID: repo.ID, UserID: user.ID})

	assert.NoError(t, DeleteCollaboration(db.DefaultContext, repo, user))
	unittest.AssertNotExistsBean(t, &repo_model.Collaboration{RepoID: repo.ID, UserID: user.ID})

	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: repo.ID})
}
