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
	assert.NoError(t, user_model.CreateBadge(db.DefaultContext, badge))

	// Create test users and assign badges
	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	assert.NoError(t, user_model.AddUserBadge(db.DefaultContext, user1, badge))
	assert.NoError(t, user_model.AddUserBadge(db.DefaultContext, user2, badge))

	// Test getting users with pagination
	opts := &user_model.GetBadgeUsersOptions{
		Badge: badge,
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 1,
		},
	}

	users, count, err := user_model.GetBadgeUsers(db.DefaultContext, opts)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, count)
	assert.Len(t, users, 1)

	// Test second page
	opts.Page = 2
	users, count, err = user_model.GetBadgeUsers(db.DefaultContext, opts)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, count)
	assert.Len(t, users, 1)

	// Test with non-existent badge
	opts.Badge = &user_model.Badge{Slug: "non-existent"}
	users, count, err = user_model.GetBadgeUsers(db.DefaultContext, opts)
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)
	assert.Empty(t, users)
}
