// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestUserNotificationSettings(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	settings, err := GetUserNotificationSettings(db.DefaultContext, 1)
	assert.NoError(t, err)
	assert.NotNil(t, settings.User)
	assert.Equal(t, settings.User.ID, settings.UserID)
	assert.Equal(t, NotificationGiteaActionsFailureOnly, settings.Actions)

	assert.NoError(t, UpdateUserNotificationSettings(db.DefaultContext, &NotificationSettings{
		UserID:  1,
		Actions: NotificationGiteaActionsAll,
	}))
	settings, err = GetUserNotificationSettings(db.DefaultContext, 1)
	assert.NoError(t, err)
	assert.NotNil(t, settings.User)
	assert.Equal(t, settings.User.ID, settings.UserID)
	assert.Equal(t, NotificationGiteaActionsAll, settings.Actions)

	assert.NoError(t, UpdateUserNotificationSettings(db.DefaultContext, &NotificationSettings{
		UserID:  1,
		Actions: NotificationGiteaActionsDisabled,
	}))
	settings, err = GetUserNotificationSettings(db.DefaultContext, 1)
	assert.NoError(t, err)
	assert.NotNil(t, settings.User)
	assert.Equal(t, settings.User.ID, settings.UserID)
	assert.Equal(t, NotificationGiteaActionsDisabled, settings.Actions)
}
