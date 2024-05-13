// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestPinRepoFunctionality(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	unittest.AssertNotExistsBean(t, &repo_model.Pin{UID: user.ID, RepoID: repo.ID})
	assert.NoError(t, repo_model.PinRepo(db.DefaultContext, user, repo, true))
	unittest.AssertExistsAndLoadBean(t, &repo_model.Pin{UID: user.ID, RepoID: repo.ID})
	assert.NoError(t, repo_model.PinRepo(db.DefaultContext, user, repo, true))
	unittest.AssertExistsAndLoadBean(t, &repo_model.Pin{UID: user.ID, RepoID: repo.ID})
	assert.NoError(t, repo_model.PinRepo(db.DefaultContext, user, repo, false))
	unittest.AssertNotExistsBean(t, &repo_model.Star{UID: user.ID, RepoID: repo.ID})
}

func TestIsPinned(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	unittest.AssertNotExistsBean(t, &repo_model.Pin{UID: user.ID, RepoID: repo.ID})
	assert.NoError(t, repo_model.PinRepo(db.DefaultContext, user, repo, true))
	unittest.AssertExistsAndLoadBean(t, &repo_model.Pin{UID: user.ID, RepoID: repo.ID})

	assert.True(t, repo_model.IsPinned(db.DefaultContext, user.ID, repo.ID))

	assert.NoError(t, repo_model.PinRepo(db.DefaultContext, user, repo, false))
	unittest.AssertNotExistsBean(t, &repo_model.Star{UID: user.ID, RepoID: repo.ID})

	assert.False(t, repo_model.IsPinned(db.DefaultContext, user.ID, repo.ID))
}
