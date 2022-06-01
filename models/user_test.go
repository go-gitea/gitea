// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestFollowUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(followerID, followedID int64) {
		assert.NoError(t, user_model.FollowUser(followerID, followedID))
		unittest.AssertExistsAndLoadBean(t, &user_model.Follow{UserID: followerID, FollowID: followedID})
	}
	testSuccess(4, 2)
	testSuccess(5, 2)

	assert.NoError(t, user_model.FollowUser(2, 2))

	unittest.CheckConsistencyFor(t, &user_model.User{})
}

func TestUnfollowUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(followerID, followedID int64) {
		assert.NoError(t, user_model.UnfollowUser(followerID, followedID))
		unittest.AssertNotExistsBean(t, &user_model.Follow{UserID: followerID, FollowID: followedID})
	}
	testSuccess(4, 2)
	testSuccess(5, 2)
	testSuccess(2, 2)

	unittest.CheckConsistencyFor(t, &user_model.User{})
}
