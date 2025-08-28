// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user_test

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestSettings(t *testing.T) {
	keyName := "test_user_setting"
	assert.NoError(t, unittest.PrepareTestDatabase())

	newSetting := &user_model.Setting{UserID: 99, SettingKey: keyName, SettingValue: "Gitea User Setting Test"}

	// create setting
	err := user_model.SetUserSetting(t.Context(), newSetting.UserID, newSetting.SettingKey, newSetting.SettingValue)
	assert.NoError(t, err)
	// test about saving unchanged values
	err = user_model.SetUserSetting(t.Context(), newSetting.UserID, newSetting.SettingKey, newSetting.SettingValue)
	assert.NoError(t, err)

	// get specific setting
	settings, err := user_model.GetSettings(t.Context(), 99, []string{keyName})
	assert.NoError(t, err)
	assert.Len(t, settings, 1)
	assert.Equal(t, newSetting.SettingValue, settings[keyName].SettingValue)

	settingValue, err := user_model.GetUserSetting(t.Context(), 99, keyName)
	assert.NoError(t, err)
	assert.Equal(t, newSetting.SettingValue, settingValue)

	settingValue, err = user_model.GetUserSetting(t.Context(), 99, "no_such")
	assert.NoError(t, err)
	assert.Empty(t, settingValue)

	// updated setting
	updatedSetting := &user_model.Setting{UserID: 99, SettingKey: keyName, SettingValue: "Updated"}
	err = user_model.SetUserSetting(t.Context(), updatedSetting.UserID, updatedSetting.SettingKey, updatedSetting.SettingValue)
	assert.NoError(t, err)

	// get all settings
	settings, err = user_model.GetUserAllSettings(t.Context(), 99)
	assert.NoError(t, err)
	assert.Len(t, settings, 1)
	assert.Equal(t, updatedSetting.SettingValue, settings[updatedSetting.SettingKey].SettingValue)

	// delete setting
	err = user_model.DeleteUserSetting(t.Context(), 99, keyName)
	assert.NoError(t, err)
	settings, err = user_model.GetUserAllSettings(t.Context(), 99)
	assert.NoError(t, err)
	assert.Empty(t, settings)
}
