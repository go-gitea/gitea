// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestSettings(t *testing.T) {
	keyName := "test_user_setting"
	assert.NoError(t, unittest.PrepareTestDatabase())

	newSetting := &user_model.Setting{UserID: 99, SettingKey: keyName, SettingValue: "Gitea User Setting Test"}

	// create setting
	err := user_model.SetUserSetting(db.DefaultContext, newSetting.UserID, newSetting.SettingKey, newSetting.SettingValue)
	assert.NoError(t, err)
	// test about saving unchanged values
	err = user_model.SetUserSetting(db.DefaultContext, newSetting.UserID, newSetting.SettingKey, newSetting.SettingValue)
	assert.NoError(t, err)

	// get specific setting
	settings, err := user_model.GetSettings(db.DefaultContext, 99, []string{keyName})
	assert.NoError(t, err)
	assert.Len(t, settings, 1)
	assert.EqualValues(t, newSetting.SettingValue, settings[keyName].SettingValue)

	settingValue, err := user_model.GetUserSetting(db.DefaultContext, 99, keyName)
	assert.NoError(t, err)
	assert.EqualValues(t, newSetting.SettingValue, settingValue)

	settingValue, err = user_model.GetUserSetting(db.DefaultContext, 99, "no_such")
	assert.NoError(t, err)
	assert.EqualValues(t, "", settingValue)

	// updated setting
	updatedSetting := &user_model.Setting{UserID: 99, SettingKey: keyName, SettingValue: "Updated"}
	err = user_model.SetUserSetting(db.DefaultContext, updatedSetting.UserID, updatedSetting.SettingKey, updatedSetting.SettingValue)
	assert.NoError(t, err)

	// get all settings
	settings, err = user_model.GetUserAllSettings(db.DefaultContext, 99)
	assert.NoError(t, err)
	assert.Len(t, settings, 1)
	assert.EqualValues(t, updatedSetting.SettingValue, settings[updatedSetting.SettingKey].SettingValue)

	// delete setting
	err = user_model.DeleteUserSetting(db.DefaultContext, 99, keyName)
	assert.NoError(t, err)
	settings, err = user_model.GetUserAllSettings(db.DefaultContext, 99)
	assert.NoError(t, err)
	assert.Len(t, settings, 0)
}
