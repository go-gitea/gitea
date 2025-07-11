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
	assert.Equal(t, NotificationActionsFailureOnly, settings.Actions)

	assert.NoError(t, UpdateUserNotificationSettings(db.DefaultContext, &NotificationSettings{
		UserID:  1,
		Actions: NotificationActionsAll,
	}))
	settings, err = GetUserNotificationSettings(db.DefaultContext, 1)
	assert.NoError(t, err)
	assert.Equal(t, NotificationActionsAll, settings.Actions)

	assert.NoError(t, UpdateUserNotificationSettings(db.DefaultContext, &NotificationSettings{
		UserID:  1,
		Actions: NotificationActionsDisabled,
	}))
	settings, err = GetUserNotificationSettings(db.DefaultContext, 1)
	assert.NoError(t, err)
	assert.Equal(t, NotificationActionsDisabled, settings.Actions)
}
