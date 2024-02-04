// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestRepository_DeleteCollaboration(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	assert.NoError(t, repo.LoadOwner(db.DefaultContext))
	assert.NoError(t, DeleteCollaboration(db.DefaultContext, repo, 4))
	unittest.AssertNotExistsBean(t, &repo_model.Collaboration{RepoID: repo.ID, UserID: 4})

	assert.NoError(t, DeleteCollaboration(db.DefaultContext, repo, 4))
	unittest.AssertNotExistsBean(t, &repo_model.Collaboration{RepoID: repo.ID, UserID: 4})

	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: repo.ID})
}
