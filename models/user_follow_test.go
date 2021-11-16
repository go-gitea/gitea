// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"github.com/stretchr/testify/assert"
)

func TestIsFollowing(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	assert.True(t, IsFollowing(4, 2))
	assert.False(t, IsFollowing(2, 4))
	assert.False(t, IsFollowing(5, unittest.NonexistentID))
	assert.False(t, IsFollowing(unittest.NonexistentID, 5))
	assert.False(t, IsFollowing(unittest.NonexistentID, unittest.NonexistentID))
}

func TestFollowUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(followerID, followedID int64) {
		assert.NoError(t, FollowUser(followerID, followedID))
		unittest.AssertExistsAndLoadBean(t, &Follow{UserID: followerID, FollowID: followedID})
	}
	testSuccess(4, 2)
	testSuccess(5, 2)

	assert.NoError(t, FollowUser(2, 2))

	unittest.CheckConsistencyFor(t, &User{})
}

func TestUnfollowUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(followerID, followedID int64) {
		assert.NoError(t, UnfollowUser(followerID, followedID))
		unittest.AssertNotExistsBean(t, &Follow{UserID: followerID, FollowID: followedID})
	}
	testSuccess(4, 2)
	testSuccess(5, 2)
	testSuccess(2, 2)

	unittest.CheckConsistencyFor(t, &User{})
}
