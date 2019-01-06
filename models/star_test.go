// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStarRepo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	const userID = 2
	const repoID = 1
	AssertNotExistsBean(t, &Star{UID: userID, RepoID: repoID})
	assert.NoError(t, StarRepo(userID, repoID, true))
	AssertExistsAndLoadBean(t, &Star{UID: userID, RepoID: repoID})
	assert.NoError(t, StarRepo(userID, repoID, true))
	AssertExistsAndLoadBean(t, &Star{UID: userID, RepoID: repoID})
	assert.NoError(t, StarRepo(userID, repoID, false))
	AssertNotExistsBean(t, &Star{UID: userID, RepoID: repoID})
}

func TestIsStaring(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	assert.True(t, IsStaring(2, 4))
	assert.False(t, IsStaring(3, 4))
}

func TestRepository_GetStargazers(t *testing.T) {
	// repo with stargazers
	assert.NoError(t, PrepareTestDatabase())
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 4}).(*Repository)
	gazers, err := repo.GetStargazers(0)
	assert.NoError(t, err)
	if assert.Len(t, gazers, 1) {
		assert.Equal(t, int64(2), gazers[0].ID)
	}
}

func TestRepository_GetStargazers2(t *testing.T) {
	// repo with stargazers
	assert.NoError(t, PrepareTestDatabase())
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 3}).(*Repository)
	gazers, err := repo.GetStargazers(0)
	assert.NoError(t, err)
	assert.Len(t, gazers, 0)
}

func TestUser_GetStarredRepos(t *testing.T) {
	// user who has starred repos
	assert.NoError(t, PrepareTestDatabase())

	user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	starred, err := user.GetStarredRepos(false, 1, 10, "")
	assert.NoError(t, err)
	if assert.Len(t, starred, 1) {
		assert.Equal(t, int64(4), starred[0].ID)
	}

	starred, err = user.GetStarredRepos(true, 1, 10, "")
	assert.NoError(t, err)
	if assert.Len(t, starred, 2) {
		assert.Equal(t, int64(2), starred[0].ID)
		assert.Equal(t, int64(4), starred[1].ID)
	}
}

func TestUser_GetStarredRepos2(t *testing.T) {
	// user who has no starred repos
	assert.NoError(t, PrepareTestDatabase())

	user := AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	starred, err := user.GetStarredRepos(false, 1, 10, "")
	assert.NoError(t, err)
	assert.Len(t, starred, 0)

	starred, err = user.GetStarredRepos(true, 1, 10, "")
	assert.NoError(t, err)
	assert.Len(t, starred, 0)
}

func TestUserGetStarredRepoCount(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	counts, err := user.GetStarredRepoCount(false)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), counts)

	counts, err = user.GetStarredRepoCount(true)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), counts)
}
