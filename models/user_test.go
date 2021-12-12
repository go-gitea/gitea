// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
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

func TestUserIsPublicMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	tt := []struct {
		uid      int64
		orgid    int64
		expected bool
	}{
		{2, 3, true},
		{4, 3, false},
		{5, 6, true},
		{5, 7, false},
	}
	for _, v := range tt {
		t.Run(fmt.Sprintf("UserId%dIsPublicMemberOf%d", v.uid, v.orgid), func(t *testing.T) {
			testUserIsPublicMember(t, v.uid, v.orgid, v.expected)
		})
	}
}

func testUserIsPublicMember(t *testing.T, uid, orgID int64, expected bool) {
	user, err := user_model.GetUserByID(uid)
	assert.NoError(t, err)
	is, err := IsPublicMembership(orgID, user.ID)
	assert.NoError(t, err)
	assert.Equal(t, expected, is)
}

func TestIsUserOrgOwner(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	tt := []struct {
		uid      int64
		orgid    int64
		expected bool
	}{
		{2, 3, true},
		{4, 3, false},
		{5, 6, true},
		{5, 7, true},
	}
	for _, v := range tt {
		t.Run(fmt.Sprintf("UserId%dIsOrgOwnerOf%d", v.uid, v.orgid), func(t *testing.T) {
			testIsUserOrgOwner(t, v.uid, v.orgid, v.expected)
		})
	}
}

func testIsUserOrgOwner(t *testing.T, uid, orgID int64, expected bool) {
	user, err := user_model.GetUserByID(uid)
	assert.NoError(t, err)
	is, err := IsOrganizationOwner(orgID, user.ID)
	assert.NoError(t, err)
	assert.Equal(t, expected, is)
}

func TestGetOrgRepositoryIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}).(*user_model.User)
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}).(*user_model.User)

	accessibleRepos, err := GetOrgRepositoryIDs(user2)
	assert.NoError(t, err)
	// User 2's team has access to private repos 3, 5, repo 32 is a public repo of the organization
	assert.Equal(t, []int64{3, 5, 23, 24, 32}, accessibleRepos)

	accessibleRepos, err = GetOrgRepositoryIDs(user4)
	assert.NoError(t, err)
	// User 4's team has access to private repo 3, repo 32 is a public repo of the organization
	assert.Equal(t, []int64{3, 32}, accessibleRepos)

	accessibleRepos, err = GetOrgRepositoryIDs(user5)
	assert.NoError(t, err)
	// User 5's team has no access to any repo
	assert.Len(t, accessibleRepos, 0)
}
