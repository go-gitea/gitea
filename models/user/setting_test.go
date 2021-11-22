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

	// updated setting
	updatedSetting := &Setting{UserID: 99, SettingKey: keyName, SettingValue: "Updated"}
	err = SetSetting(updatedSetting)
	assert.NoError(t, err)

	// get all settings
	settings, err = GetUserAllSettings(99)
	assert.NoError(t, err)
	assert.Len(t, settings, 1)
	assert.EqualValues(t, updatedSetting.SettingValue, settings[updatedSetting.SettingKey].SettingValue)

	// delete setting
	err = DeleteSetting(&Setting{UserID: 99, SettingKey: keyName})
	assert.NoError(t, err)
	settings, err = GetUserAllSettings(99)
	assert.NoError(t, err)
	assert.Len(t, settings, 0)
}
