// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestSettings(t *testing.T) {
	keyName := "test_repo_setting"
	assert.NoError(t, unittest.PrepareTestDatabase())

	newSetting := &Setting{RepoID: 99, SettingKey: keyName, SettingValue: "Gitea Repo Setting Test"}

	// create setting
	err := SetSetting(newSetting)
	assert.NoError(t, err)
	// test about saving unchanged values
	err = SetSetting(newSetting)
	assert.NoError(t, err)

	// get specific setting
	settings, err := GetSettings(99, []string{keyName})
	assert.NoError(t, err)
	assert.Len(t, settings, 1)
	assert.EqualValues(t, newSetting.SettingValue, settings[keyName].SettingValue)

	settingValue, err := GetSetting(99, keyName)
	assert.NoError(t, err)
	assert.EqualValues(t, newSetting.SettingValue, settingValue)

	settingValue, err = GetSetting(99, "no_such")
	assert.NoError(t, err)
	assert.EqualValues(t, "", settingValue)

	// updated setting
	updatedSetting := &Setting{RepoID: 99, SettingKey: keyName, SettingValue: "Updated"}
	err = SetSetting(updatedSetting)
	assert.NoError(t, err)

	// get all settings
	settings, err = GetAllSettings(99)
	assert.NoError(t, err)
	assert.Len(t, settings, 1)
	assert.EqualValues(t, updatedSetting.SettingValue, settings[updatedSetting.SettingKey].SettingValue)

	// delete setting
	err = DeleteSetting(99, keyName)
	assert.NoError(t, err)
	settings, err = GetAllSettings(99)
	assert.NoError(t, err)
	assert.Len(t, settings, 0)
}
