// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddPinnedRepo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user2 := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	repo1 := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	repo10 := AssertExistsAndLoadBean(t, &Repository{ID: 10}).(*Repository)

	assert.NoError(t, user2.AddPinnedRepo(repo10))
	AssertExistsAndLoadBean(t, &UserPinnedRepo{UID: user2.ID, RepoID: repo10.ID})

	assert.EqualError(t, user2.AddPinnedRepo(repo1),
		ErrUserPinnedRepoAlreadyExist{UID: user2.ID, RepoID: repo1.ID}.Error())
}

func TestRemovePinnedRepo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user2 := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)

	assert.NoError(t, user2.RemovePinnedRepo(1))

	assert.EqualError(t, user2.RemovePinnedRepo(3),
		ErrUserPinnedRepoNotExist{UID: user2.ID, RepoID: 3}.Error())
}

func TestIsPinnedRepoExist(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user2 := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)

	exist, err := user2.IsPinnedRepoExist(1)
	assert.NoError(t, err)
	assert.Equal(t, true, exist)

	exist, err = user2.IsPinnedRepoExist(5)
	assert.NoError(t, err)
	assert.Equal(t, false, exist)
}

func TestGetPinnedRepos(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user2 := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)

	repoIDs, err := user2.GetPinnedRepoIDs(user2)
	assert.NoError(t, err)
	assert.Equal(t, []int64{1, 4, 16}, repoIDs)

	repoIDs, err = user2.GetPinnedRepoIDs(nil)
	assert.NoError(t, err)
	assert.Equal(t, []int64{1, 4}, repoIDs)
}
