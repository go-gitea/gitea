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
	keyName := "test_user_setting"
	assert.NoError(t, db.PrepareTestDatabase())

	newSetting := &UserSetting{UserID: 99, SettingKey: keyName, SettingValue: "Gitea User Setting Test"}

	// create setting
	err := SetUserSetting(newSetting)
	assert.NoError(t, err)

	// get specific setting
	userSettings, err := GetUserSetting(99, []string{keyName})
	assert.NoError(t, err)
	assert.Len(t, userSettings, 1)
	assert.EqualValues(t, newSetting.SettingValue, userSettings[keyName].SettingValue)

	// updated setting
	updatedSetting := &UserSetting{UserID: 99, SettingKey: keyName, SettingValue: "Updated", ID: userSettings[keyName].ID}
	err = SetUserSetting(updatedSetting)
	assert.NoError(t, err)

	// get all settings
	userSettings, err = GetUserAllSettings(99)
	assert.NoError(t, err)
	assert.Len(t, userSettings, 1)
	assert.EqualValues(t, userSettings[updatedSetting.SettingKey].SettingValue, updatedSetting.SettingValue)

	// delete setting
	err = DeleteUserSetting(updatedSetting)
	assert.NoError(t, err)
}
