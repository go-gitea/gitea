// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestCanBlockUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	user29 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 29})
	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})

	// Doer can't self block
	assert.False(t, CanBlockUser(db.DefaultContext, user1, user2, user1))
	// Blocker can't be blockee
	assert.False(t, CanBlockUser(db.DefaultContext, user1, user2, user2))
	// Can't block already blocked user
	assert.False(t, CanBlockUser(db.DefaultContext, user1, user2, user29))
	// Blockee can't be an organization
	assert.False(t, CanBlockUser(db.DefaultContext, user1, user2, org3))
	// Doer must be blocker or admin
	assert.False(t, CanBlockUser(db.DefaultContext, user2, user4, user29))
	// Organization can't block a member
	assert.False(t, CanBlockUser(db.DefaultContext, user1, org3, user4))
	// Doer must be organization owner or admin if blocker is an organization
	assert.False(t, CanBlockUser(db.DefaultContext, user4, org3, user2))

	assert.True(t, CanBlockUser(db.DefaultContext, user1, user2, user4))
	assert.True(t, CanBlockUser(db.DefaultContext, user2, user2, user4))
	assert.True(t, CanBlockUser(db.DefaultContext, user2, org3, user29))
}

func TestCanUnblockUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user28 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 28})
	user29 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 29})
	org17 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 17})

	// Doer can't self unblock
	assert.False(t, CanUnblockUser(db.DefaultContext, user1, user2, user1))
	// Can't unblock not blocked user
	assert.False(t, CanUnblockUser(db.DefaultContext, user1, user2, user28))
	// Doer must be blocker or admin
	assert.False(t, CanUnblockUser(db.DefaultContext, user28, user2, user29))
	// Doer must be organization owner or admin if blocker is an organization
	assert.False(t, CanUnblockUser(db.DefaultContext, user2, org17, user28))

	assert.True(t, CanUnblockUser(db.DefaultContext, user1, user2, user29))
	assert.True(t, CanUnblockUser(db.DefaultContext, user2, user2, user29))
	assert.True(t, CanUnblockUser(db.DefaultContext, user1, org17, user28))
}
