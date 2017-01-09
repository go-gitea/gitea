// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

var accessModes = []AccessMode{
	AccessModeRead,
	AccessModeWrite,
	AccessModeAdmin,
	AccessModeOwner,
}

func TestAccessLevel(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user1 := &User{ID: 2}; AssertExistsAndLoadBean(t, user1)
	user2 := &User{ID: 4}; AssertExistsAndLoadBean(t, user2)
	repo1 := &Repository{OwnerID: 2, IsPrivate: false}; AssertExistsAndLoadBean(t, repo1)
	repo2 := &Repository{OwnerID: 3, IsPrivate: true}; AssertExistsAndLoadBean(t, repo2)

	level, err := AccessLevel(user1, repo1)
	assert.NoError(t, err)
	assert.Equal(t, AccessModeOwner, level)

	level, err = AccessLevel(user1, repo2)
	assert.NoError(t, err)
	assert.Equal(t, AccessModeWrite, level)

	level, err = AccessLevel(user2, repo1)
	assert.NoError(t, err)
	assert.Equal(t, AccessModeRead, level)

	level, err = AccessLevel(user2, repo2)
	assert.NoError(t, err)
	assert.Equal(t, AccessModeNone, level)
}

func TestHasAccess(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user1 := &User{ID: 2}; AssertExistsAndLoadBean(t, user1)
	user2 := &User{ID: 4}; AssertExistsAndLoadBean(t, user2)
	repo1 := &Repository{OwnerID: 2, IsPrivate: false}; AssertExistsAndLoadBean(t, repo1)
	repo2 := &Repository{OwnerID: 3, IsPrivate: true}; AssertExistsAndLoadBean(t, repo2)

	for _, accessMode := range accessModes {
		has, err := HasAccess(user1, repo1, accessMode)
		assert.NoError(t, err)
		assert.True(t, has)

		has, err = HasAccess(user1, repo2, accessMode)
		assert.NoError(t, err)
		assert.Equal(t, accessMode <= AccessModeWrite, has)

		has, err = HasAccess(user2, repo1, accessMode)
		assert.NoError(t, err)
		assert.Equal(t, accessMode <= AccessModeRead, has)

		has, err = HasAccess(user2, repo2, accessMode)
		assert.NoError(t, err)
		assert.Equal(t, accessMode <= AccessModeNone, has)
	}
}

func TestUser_GetRepositoryAccesses(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user1 := &User{ID: 1}; AssertExistsAndLoadBean(t, user1)
	user2 := &User{ID: 2}; AssertExistsAndLoadBean(t, user2)

	accesses, err := user1.GetRepositoryAccesses()
	assert.NoError(t, err)
	assert.Len(t, accesses, 0)
}

func TestUser_GetAccessibleRepositories(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user1 := &User{ID: 1}; AssertExistsAndLoadBean(t, user1)
	user2 := &User{ID: 2}; AssertExistsAndLoadBean(t, user2)

	repos, err := user1.GetAccessibleRepositories(0)
	assert.NoError(t, err)
	assert.Len(t, repos, 0)

	repos, err = user2.GetAccessibleRepositories(0)
	assert.NoError(t, err)
	assert.Len(t, repos, 1)
}


func TestRepository_RecalculateAccesses(t *testing.T) {
	// test with organization repo
	assert.NoError(t, PrepareTestDatabase())
	repo1 := &Repository{ID: 3}; AssertExistsAndLoadBean(t, repo1)
	assert.NoError(t, repo1.GetOwner())

	sess := x.NewSession()
	defer sess.Close()
	_, err := sess.Delete(&Collaboration{UserID: 2, RepoID: 3})
	assert.NoError(t, err)

	assert.NoError(t, repo1.RecalculateAccesses())

	sess = x.NewSession()
	access := &Access{UserID: 2, RepoID: 3}
	has, err := sess.Get(access)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, AccessModeWrite, access.Mode)
}

func TestRepository_RecalculateAccesses2(t *testing.T) {
	// test with non-organization repo
	assert.NoError(t, PrepareTestDatabase())
	repo1 := &Repository{ID: 4}; AssertExistsAndLoadBean(t, repo1)
	assert.NoError(t, repo1.GetOwner())

	sess := x.NewSession()
	defer sess.Close()
	_, err := sess.Delete(&Collaboration{UserID: 4, RepoID: 4})
	assert.NoError(t, err)

	assert.NoError(t, repo1.RecalculateAccesses())

	sess = x.NewSession()
	has, err := sess.Get(&Access{UserID: 4, RepoID: 4})
	assert.NoError(t, err)
	assert.False(t, has)
}
