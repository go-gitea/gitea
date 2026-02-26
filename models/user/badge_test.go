// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestGetBadgeNotExist(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	badge, err := user_model.GetBadge(t.Context(), "does-not-exist")
	assert.Nil(t, badge)
	assert.Error(t, err)
	assert.True(t, user_model.IsErrBadgeNotExist(err))
}

func TestCreateBadgeAlreadyExists(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	badge := &user_model.Badge{
		Slug:        "duplicate-badge-slug",
		Description: "First",
	}
	assert.NoError(t, user_model.CreateBadge(t.Context(), badge))

	err := user_model.CreateBadge(t.Context(), &user_model.Badge{
		Slug:        "duplicate-badge-slug",
		Description: "Second",
	})
	assert.Error(t, err)
	assert.True(t, user_model.IsErrBadgeAlreadyExist(err))
}

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

func TestSearchBadgesOrderingAndKeyword(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	createdBadges := []*user_model.Badge{
		{Slug: "badge-sort-b", Description: "Badge Sort B"},
		{Slug: "badge-sort-c", Description: "Badge Sort C"},
		{Slug: "badge-sort-a", Description: "Badge Sort A"},
		{Slug: "badge-sort-case", Description: "MiXeDCaSeKeyword"},
	}
	for _, badge := range createdBadges {
		assert.NoError(t, user_model.CreateBadge(t.Context(), badge))
	}

	opts := &user_model.SearchBadgeOptions{
		ListOptions: db.ListOptions{ListAll: true},
		Keyword:     "badge-sort-",
		OrderBy:     db.SearchOrderBy("`badge`.id ASC"),
	}

	oldestFirst, count, err := user_model.SearchBadges(t.Context(), opts)
	assert.NoError(t, err)
	assert.EqualValues(t, 4, count)
	assert.Equal(t, []string{"badge-sort-b", "badge-sort-c", "badge-sort-a", "badge-sort-case"}, collectBadgeSlugs(oldestFirst))

	opts.OrderBy = db.SearchOrderBy("`badge`.id DESC")
	newestFirst, count, err := user_model.SearchBadges(t.Context(), opts)
	assert.NoError(t, err)
	assert.EqualValues(t, 4, count)
	assert.Equal(t, []string{"badge-sort-case", "badge-sort-a", "badge-sort-c", "badge-sort-b"}, collectBadgeSlugs(newestFirst))

	opts.OrderBy = db.SearchOrderBy("`badge`.slug ASC")
	alpha, count, err := user_model.SearchBadges(t.Context(), opts)
	assert.NoError(t, err)
	assert.EqualValues(t, 4, count)
	assert.Equal(t, []string{"badge-sort-a", "badge-sort-b", "badge-sort-c", "badge-sort-case"}, collectBadgeSlugs(alpha))

	opts.OrderBy = db.SearchOrderBy("`badge`.slug DESC")
	reverseAlpha, count, err := user_model.SearchBadges(t.Context(), opts)
	assert.NoError(t, err)
	assert.EqualValues(t, 4, count)
	assert.Equal(t, []string{"badge-sort-case", "badge-sort-c", "badge-sort-b", "badge-sort-a"}, collectBadgeSlugs(reverseAlpha))

	opts.Keyword = "mixedcasekeyword"
	opts.OrderBy = db.SearchOrderBy("`badge`.slug ASC")
	caseInsensitive, count, err := user_model.SearchBadges(t.Context(), opts)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, count)
	assert.Equal(t, []string{"badge-sort-case"}, collectBadgeSlugs(caseInsensitive))
}

func collectBadgeSlugs(badges []*user_model.Badge) []string {
	slugs := make([]string, 0, len(badges))
	for _, badge := range badges {
		slugs = append(slugs, badge.Slug)
	}
	return slugs
}
