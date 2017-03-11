package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsFollowing(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	assert.True(t, IsFollowing(4, 2))
	assert.False(t, IsFollowing(2, 4))
	assert.False(t, IsFollowing(5, NonexistentID))
	assert.False(t, IsFollowing(NonexistentID, 5))
	assert.False(t, IsFollowing(NonexistentID, NonexistentID))
}

func TestFollowUser(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	testSuccess := func(followerID, followedID int64) {
		assert.NoError(t, FollowUser(followerID, followedID))
		AssertExistsAndLoadBean(t, &Follow{UserID: followerID, FollowID: followedID})
	}
	testSuccess(4, 2)
	testSuccess(5, 2)

	assert.NoError(t, FollowUser(2, 2))

	CheckConsistencyFor(t, &User{})
}

func TestUnfollowUser(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	testSuccess := func(followerID, followedID int64) {
		assert.NoError(t, UnfollowUser(followerID, followedID))
		AssertNotExistsBean(t, &Follow{UserID: followerID, FollowID: followedID})
	}
	testSuccess(4, 2)
	testSuccess(5, 2)
	testSuccess(2, 2)

	CheckConsistencyFor(t, &User{})
}
