// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestStarRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	const userID = 2
	const repoID = 1
	unittest.AssertNotExistsBean(t, &repo_model.Star{UID: userID, RepoID: repoID})
	assert.NoError(t, repo_model.StarRepo(db.DefaultContext, userID, repoID, true))
	unittest.AssertExistsAndLoadBean(t, &repo_model.Star{UID: userID, RepoID: repoID})
	assert.NoError(t, repo_model.StarRepo(db.DefaultContext, userID, repoID, true))
	unittest.AssertExistsAndLoadBean(t, &repo_model.Star{UID: userID, RepoID: repoID})
	assert.NoError(t, repo_model.StarRepo(db.DefaultContext, userID, repoID, false))
	unittest.AssertNotExistsBean(t, &repo_model.Star{UID: userID, RepoID: repoID})
}

func TestIsStaring(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	assert.True(t, repo_model.IsStaring(db.DefaultContext, 2, 4))
	assert.False(t, repo_model.IsStaring(db.DefaultContext, 3, 4))
}

func TestRepository_GetStargazers(t *testing.T) {
	// repo with stargazers
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	gazers, err := repo_model.GetStargazers(db.DefaultContext, repo, db.ListOptions{Page: 0})
	assert.NoError(t, err)
	if assert.Len(t, gazers, 1) {
		assert.Equal(t, int64(2), gazers[0].ID)
	}
}

func TestRepository_GetStargazers2(t *testing.T) {
	// repo with stargazers
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	gazers, err := repo_model.GetStargazers(db.DefaultContext, repo, db.ListOptions{Page: 0})
	assert.NoError(t, err)
	assert.Len(t, gazers, 0)
}

func TestClearRepoStars(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	const userID = 2
	const repoID = 1
	unittest.AssertNotExistsBean(t, &repo_model.Star{UID: userID, RepoID: repoID})
	assert.NoError(t, repo_model.StarRepo(db.DefaultContext, userID, repoID, true))
	unittest.AssertExistsAndLoadBean(t, &repo_model.Star{UID: userID, RepoID: repoID})
	assert.NoError(t, repo_model.StarRepo(db.DefaultContext, userID, repoID, false))
	unittest.AssertNotExistsBean(t, &repo_model.Star{UID: userID, RepoID: repoID})
	assert.NoError(t, repo_model.ClearRepoStars(db.DefaultContext, repoID))
	unittest.AssertNotExistsBean(t, &repo_model.Star{UID: userID, RepoID: repoID})

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	gazers, err := repo_model.GetStargazers(db.DefaultContext, repo, db.ListOptions{Page: 0})
	assert.NoError(t, err)
	assert.Len(t, gazers, 0)
}
