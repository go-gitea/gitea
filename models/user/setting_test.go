// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestSettings(t *testing.T) {
	keyName := "test_user_setting"
	assert.NoError(t, unittest.PrepareTestDatabase())

	newSetting := &Setting{UserID: 99, SettingKey: keyName, SettingValue: "Gitea User Setting Test"}

	// create setting
	err := SetUserSetting(newSetting.UserID, newSetting.SettingKey, newSetting.SettingValue)
	assert.NoError(t, err)
	// test about saving unchanged values
	err = SetUserSetting(newSetting.UserID, newSetting.SettingKey, newSetting.SettingValue)
	assert.NoError(t, err)

	// get specific setting
	settings, err := GetUserSettings(99, []string{keyName})
	assert.NoError(t, err)
	assert.Len(t, settings, 1)
	assert.EqualValues(t, newSetting.SettingValue, settings[keyName].SettingValue)

	settingValue, err := GetUserSetting(99, keyName)
	assert.NoError(t, err)
	assert.EqualValues(t, newSetting.SettingValue, settingValue)

	settingValue, err = GetUserSetting(99, "no_such")
	assert.NoError(t, err)
	assert.EqualValues(t, "", settingValue)

	// updated setting
	updatedSetting := &Setting{UserID: 99, SettingKey: keyName, SettingValue: "Updated"}
	err = SetUserSetting(updatedSetting.UserID, updatedSetting.SettingKey, updatedSetting.SettingValue)
	assert.NoError(t, err)

	// get all settings
	settings, err = GetUserAllSettings(99)
	assert.NoError(t, err)
	assert.Len(t, settings, 1)
	assert.EqualValues(t, updatedSetting.SettingValue, settings[updatedSetting.SettingKey].SettingValue)

	// delete setting
	err = DeleteUserSetting(99, keyName)
	assert.NoError(t, err)
	settings, err = GetUserAllSettings(99)
	assert.NoError(t, err)
	assert.Len(t, settings, 0)
}
