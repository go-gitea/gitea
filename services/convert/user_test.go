// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestUser_ToUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1, IsAdmin: true})

	apiUser := toUser(t.Context(), user1, true, true)
	assert.True(t, apiUser.IsAdmin)
	assert.Contains(t, apiUser.AvatarURL, "://")

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2, IsAdmin: false})

	apiUser = toUser(t.Context(), user2, true, true)
	assert.False(t, apiUser.IsAdmin)

	apiUser = toUser(t.Context(), user1, false, false)
	assert.False(t, apiUser.IsAdmin)
	assert.Equal(t, api.VisibleTypePublic.String(), apiUser.Visibility)

	user31 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 31, IsAdmin: false, Visibility: api.VisibleTypePrivate})

	apiUser = toUser(t.Context(), user31, true, true)
	assert.False(t, apiUser.IsAdmin)
	assert.Equal(t, api.VisibleTypePrivate.String(), apiUser.Visibility)
}
