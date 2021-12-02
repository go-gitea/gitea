// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestStarRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	const userID = 2
	const repoID = 1
	unittest.AssertNotExistsBean(t, &Star{UID: userID, RepoID: repoID})
	assert.NoError(t, StarRepo(userID, repoID, true))
	unittest.AssertExistsAndLoadBean(t, &Star{UID: userID, RepoID: repoID})
	assert.NoError(t, StarRepo(userID, repoID, true))
	unittest.AssertExistsAndLoadBean(t, &Star{UID: userID, RepoID: repoID})
	assert.NoError(t, StarRepo(userID, repoID, false))
	unittest.AssertNotExistsBean(t, &Star{UID: userID, RepoID: repoID})
}

func TestIsStaring(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	assert.True(t, IsStaring(2, 4))
	assert.False(t, IsStaring(3, 4))
}

func TestRepository_GetStargazers(t *testing.T) {
	// repo with stargazers
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 4}).(*Repository)
	gazers, err := GetStargazers(repo, db.ListOptions{Page: 0})
	assert.NoError(t, err)
	if assert.Len(t, gazers, 1) {
		assert.Equal(t, int64(2), gazers[0].ID)
	}
}

func TestRepository_GetStargazers2(t *testing.T) {
	// repo with stargazers
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 3}).(*Repository)
	gazers, err := GetStargazers(repo, db.ListOptions{Page: 0})
	assert.NoError(t, err)
	assert.Len(t, gazers, 0)
}
