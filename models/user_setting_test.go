// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserSettings(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	newSetting := &UserSetting{UserID: 99, Key: "test_user_setting", Value: "Gitea User Setting Test"}

	// create setting
	err := AddUserSetting(newSetting)
	assert.NoError(t, err)

	// get specific setting
	userSetting, err := GetUserSetting(99, "test_user_setting")
	assert.NoError(t, err)
	assert.EqualValues(t, newSetting.Value, userSetting.Value)

	// updated setting
	updatedSetting := &UserSetting{UserID: 99, Key: "test_user_setting", Value: "Updated", ID: userSetting.ID}
	err = UpdateUserSettingValue(updatedSetting)
	assert.NoError(t, err)

	// get all settings
	userSettings, err := GetUserAllSettings(99)
	assert.NoError(t, err)
	assert.Len(t, userSettings, 1)
	assert.EqualValues(t, userSettings[0].Value, updatedSetting.Value)

	// delete setting
	err = DeleteUserSetting(updatedSetting)
	assert.NoError(t, err)
}
