// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"

	"github.com/stretchr/testify/assert"
)

func TestUserSettings(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	newSetting := &UserSetting{UserID: 99, Key: "test_user_setting", Value: "Gitea User Setting Test"}

	// create setting
	err := SetUserSetting(newSetting)
	assert.NoError(t, err)

	// get specific setting
	userSettings, err := GetUserSetting(99, []string{"test_user_setting"})
	assert.NoError(t, err)
	assert.Len(t, userSettings, 1)
	assert.EqualValues(t, newSetting.Value, userSettings[0].Value)

	// updated setting
	updatedSetting := &UserSetting{UserID: 99, Key: "test_user_setting", Value: "Updated", ID: userSettings[0].ID}
	err = SetUserSetting(updatedSetting)
	assert.NoError(t, err)

	// get all settings
	userSettings, err = GetUserAllSettings(99)
	assert.NoError(t, err)
	assert.Len(t, userSettings, 1)
	assert.EqualValues(t, userSettings[0].Value, updatedSetting.Value)

	// delete setting
	err = DeleteUserSetting(updatedSetting)
	assert.NoError(t, err)
}
