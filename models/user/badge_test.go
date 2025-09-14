// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestGetBadgeUsers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Create a test badge
	badge := &user_model.Badge{
		Slug:        "test-badge",
		Description: "Test Badge",
		ImageURL:    "test.png",
	}
	assert.NoError(t, user_model.CreateBadge(t.Context(), badge))

	// Create test users and assign badges
	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	assert.NoError(t, user_model.AddUserBadge(t.Context(), user1, badge))
	assert.NoError(t, user_model.AddUserBadge(t.Context(), user2, badge))

	// Test getting users with pagination
	opts := &user_model.GetBadgeUsersOptions{
		BadgeSlug: badge.Slug,
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 1,
		},
	}

	users, count, err := user_model.GetBadgeUsers(t.Context(), opts)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, count)
	assert.Len(t, users, 1)

	// Test second page
	opts.Page = 2
	users, count, err = user_model.GetBadgeUsers(t.Context(), opts)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, count)
	assert.Len(t, users, 1)

	// Test with non-existent badge
	opts.BadgeSlug = "non-existent"
	users, count, err = user_model.GetBadgeUsers(t.Context(), opts)
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)
	assert.Empty(t, users)
}

func TestAddAndRemoveUserBadges(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	badge1 := unittest.AssertExistsAndLoadBean(t, &user_model.Badge{ID: 1})
	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	// Add a badge to user and verify that it is returned in the list
	assert.NoError(t, user_model.AddUserBadge(t.Context(), user1, badge1))
	badges, count, err := user_model.GetUserBadges(t.Context(), user1)
	assert.Equal(t, int64(1), count)
	assert.Equal(t, badge1.Slug, badges[0].Slug)
	assert.NoError(t, err)

	// Confirm that it is impossible to duplicate the same badge
	assert.Error(t, user_model.AddUserBadge(t.Context(), user1, badge1))

	// Nothing happened to the existing badge
	badges, count, err = user_model.GetUserBadges(t.Context(), user1)
	assert.Equal(t, int64(1), count)
	assert.Equal(t, badge1.Slug, badges[0].Slug)
	assert.NoError(t, err)

	// Remove a badge from user and verify that it is no longer in the list
	assert.NoError(t, user_model.RemoveUserBadge(t.Context(), user1, badge1))
	_, count, err = user_model.GetUserBadges(t.Context(), user1)
	assert.Equal(t, int64(0), count)
	assert.NoError(t, err)
}
