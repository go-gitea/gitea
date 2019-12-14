// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccessLevel(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user2 := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	user5 := AssertExistsAndLoadBean(t, &User{ID: 5}).(*User)
	// A public repository owned by User 2
	repo1 := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	assert.False(t, repo1.IsPrivate)
	// A private repository owned by Org 3
	repo3 := AssertExistsAndLoadBean(t, &Repository{ID: 3}).(*Repository)
	assert.True(t, repo3.IsPrivate)

	level, err := AccessLevel(user2, repo1)
	assert.NoError(t, err)
	assert.Equal(t, AccessModeOwner, level)

	level, err = AccessLevel(user2, repo3)
	assert.NoError(t, err)
	assert.Equal(t, AccessModeOwner, level)

	level, err = AccessLevel(user5, repo1)
	assert.NoError(t, err)
	assert.Equal(t, AccessModeRead, level)

	level, err = AccessLevel(user5, repo3)
	assert.NoError(t, err)
	assert.Equal(t, AccessModeNone, level)
}

func TestHasAccess(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user1 := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	user2 := AssertExistsAndLoadBean(t, &User{ID: 5}).(*User)
	// A public repository owned by User 2
	repo1 := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	assert.False(t, repo1.IsPrivate)
	// A private repository owned by Org 3
	repo2 := AssertExistsAndLoadBean(t, &Repository{ID: 3}).(*Repository)
	assert.True(t, repo2.IsPrivate)

	has, err := HasAccess(user1.ID, repo1)
	assert.NoError(t, err)
	assert.True(t, has)

	_, err = HasAccess(user1.ID, repo2)
	assert.NoError(t, err)

	_, err = HasAccess(user2.ID, repo1)
	assert.NoError(t, err)

	_, err = HasAccess(user2.ID, repo2)
	assert.NoError(t, err)
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
