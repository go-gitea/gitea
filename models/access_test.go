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

	user1 := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	user2 := AssertExistsAndLoadBean(t, &User{ID: 5}).(*User)
	repo1 := AssertExistsAndLoadBean(t, &Repository{OwnerID: 2, IsPrivate: false}).(*Repository)
	repo2 := AssertExistsAndLoadBean(t, &Repository{OwnerID: 3, IsPrivate: true}).(*Repository)

	level, err := AccessLevel(user1.ID, repo1)
	assert.NoError(t, err)
	assert.Equal(t, AccessModeOwner, level)

	level, err = AccessLevel(user1.ID, repo2)
	assert.NoError(t, err)
	assert.Equal(t, AccessModeWrite, level)

	level, err = AccessLevel(user2.ID, repo1)
	assert.NoError(t, err)
	assert.Equal(t, AccessModeRead, level)

	level, err = AccessLevel(user2.ID, repo2)
	assert.NoError(t, err)
	assert.Equal(t, AccessModeNone, level)
}

func TestHasAccess(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user1 := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	user2 := AssertExistsAndLoadBean(t, &User{ID: 5}).(*User)
	repo1 := AssertExistsAndLoadBean(t, &Repository{OwnerID: 2, IsPrivate: false}).(*Repository)
	repo2 := AssertExistsAndLoadBean(t, &Repository{OwnerID: 3, IsPrivate: true}).(*Repository)

	for _, accessMode := range accessModes {
		has, err := HasAccess(user1.ID, repo1, accessMode)
		assert.NoError(t, err)
		assert.True(t, has)

		has, err = HasAccess(user1.ID, repo2, accessMode)
		assert.NoError(t, err)
		assert.Equal(t, accessMode <= AccessModeWrite, has)

		has, err = HasAccess(user2.ID, repo1, accessMode)
		assert.NoError(t, err)
		assert.Equal(t, accessMode <= AccessModeRead, has)

		has, err = HasAccess(user2.ID, repo2, accessMode)
		assert.NoError(t, err)
		assert.Equal(t, accessMode <= AccessModeNone, has)
	}
}

func TestUser_GetRepositoryAccesses(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user1 := AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	accesses, err := user1.GetRepositoryAccesses()
	assert.NoError(t, err)
	assert.Len(t, accesses, 0)
}

func TestUser_GetAccessibleRepositories(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user1 := AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	repos, err := user1.GetAccessibleRepositories(0)
	assert.NoError(t, err)
	assert.Len(t, repos, 0)

	user2 := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	repos, err = user2.GetAccessibleRepositories(0)
	assert.NoError(t, err)
	assert.Len(t, repos, 1)
}

func TestRepository_RecalculateAccesses(t *testing.T) {
	// test with organization repo
	assert.NoError(t, PrepareTestDatabase())
	repo1 := AssertExistsAndLoadBean(t, &Repository{ID: 3}).(*Repository)
	assert.NoError(t, repo1.GetOwner())

	_, err := x.Delete(&Collaboration{UserID: 2, RepoID: 3})
	assert.NoError(t, err)
	assert.NoError(t, repo1.RecalculateAccesses())

	access := &Access{UserID: 2, RepoID: 3}
	has, err := x.Get(access)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, AccessModeOwner, access.Mode)
}

func TestRepository_RecalculateAccesses2(t *testing.T) {
	// test with non-organization repo
	assert.NoError(t, PrepareTestDatabase())
	repo1 := AssertExistsAndLoadBean(t, &Repository{ID: 4}).(*Repository)
	assert.NoError(t, repo1.GetOwner())

	_, err := x.Delete(&Collaboration{UserID: 4, RepoID: 4})
	assert.NoError(t, err)
	assert.NoError(t, repo1.RecalculateAccesses())

	has, err := x.Get(&Access{UserID: 4, RepoID: 4})
	assert.NoError(t, err)
	assert.False(t, has)
}
