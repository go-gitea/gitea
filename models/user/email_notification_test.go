// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestNotificationSettings(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	u := unittest.AssertExistsAndLoadBean(t, &User{ID: 1})

	assert.NoError(t, SetUserSetting(db.DefaultContext, u.ID, SettingsEmailNotificationGiteaActions, EmailNotificationGiteaActionsAll))
	settings, err := GetSetting(db.DefaultContext, u.ID, SettingsEmailNotificationGiteaActions)
	assert.NoError(t, err)
	assert.Equal(t, EmailNotificationGiteaActionsAll, settings)

	assert.NoError(t, SetUserSetting(db.DefaultContext, u.ID, SettingsEmailNotificationGiteaActions, EmailNotificationGiteaActionsDisabled))
	settings, err = GetSetting(db.DefaultContext, u.ID, SettingsEmailNotificationGiteaActions)
	assert.NoError(t, err)
	assert.Equal(t, EmailNotificationGiteaActionsDisabled, settings)

	assert.NoError(t, SetUserSetting(db.DefaultContext, u.ID, SettingsEmailNotificationGiteaActions, EmailNotificationGiteaActionsFailureOnly))
	settings, err = GetSetting(db.DefaultContext, u.ID, SettingsEmailNotificationGiteaActions)
	assert.NoError(t, err)
	assert.Equal(t, EmailNotificationGiteaActionsFailureOnly, settings)
}
