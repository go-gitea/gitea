// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestRepository_AddCollaborator(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(repoID, userID int64) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID}).(*repo_model.Repository)
		assert.NoError(t, repo.GetOwner(db.DefaultContext))
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userID}).(*user_model.User)
		assert.NoError(t, AddCollaborator(repo, user))
		unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: repoID}, &user_model.User{ID: userID})
	}
	testSuccess(1, 4)
	testSuccess(1, 4)
	testSuccess(3, 4)
}

func TestRepository_ChangeCollaborationAccessMode(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4}).(*repo_model.Repository)
	assert.NoError(t, repo_model.ChangeCollaborationAccessMode(repo, 4, perm.AccessModeAdmin))

	collaboration := unittest.AssertExistsAndLoadBean(t, &repo_model.Collaboration{RepoID: repo.ID, UserID: 4}).(*repo_model.Collaboration)
	assert.EqualValues(t, perm.AccessModeAdmin, collaboration.Mode)

	access := unittest.AssertExistsAndLoadBean(t, &access_model.Access{UserID: 4, RepoID: repo.ID}).(*access_model.Access)
	assert.EqualValues(t, perm.AccessModeAdmin, access.Mode)

	assert.NoError(t, repo_model.ChangeCollaborationAccessMode(repo, 4, perm.AccessModeAdmin))

	assert.NoError(t, repo_model.ChangeCollaborationAccessMode(repo, unittest.NonexistentID, perm.AccessModeAdmin))

	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: repo.ID})
}

func TestRepository_DeleteCollaboration(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4}).(*repo_model.Repository)
	assert.NoError(t, repo.GetOwner(db.DefaultContext))
	assert.NoError(t, DeleteCollaboration(repo, 4))
	unittest.AssertNotExistsBean(t, &repo_model.Collaboration{RepoID: repo.ID, UserID: 4})

	assert.NoError(t, DeleteCollaboration(repo, 4))
	unittest.AssertNotExistsBean(t, &repo_model.Collaboration{RepoID: repo.ID, UserID: 4})

	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: repo.ID})
}
