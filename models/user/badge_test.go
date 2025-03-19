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

func TestAddAndRemoveUserBadges(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	badge1 := unittest.AssertExistsAndLoadBean(t, &user_model.Badge{ID: 1})
	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	// Add a badge to user and verify that it is returned in the list
	assert.NoError(t, user_model.AddUserBadge(db.DefaultContext, user1, badge1))
	badges, count, err := user_model.GetUserBadges(db.DefaultContext, user1)
	assert.Equal(t, count, int64(1))
	assert.Equal(t, badges[0].Slug, badge1.Slug)
	assert.Nil(t, err)

	// Remove a badge from user and verify that it is no longer in the list
	assert.NoError(t, user_model.RemoveUserBadge(db.DefaultContext, user1, badge1))
	badges, count, err = user_model.GetUserBadges(db.DefaultContext, user1)
	assert.Equal(t, count, int64(0))
	assert.Nil(t, err)
}
