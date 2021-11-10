// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"testing"

	"code.gitea.io/gitea/models/db"

	"github.com/stretchr/testify/assert"
)

func TestSettings(t *testing.T) {
	keyName := "test_user_setting"
	assert.NoError(t, db.PrepareTestDatabase())

	newSetting := &Setting{UserID: 99, SettingKey: keyName, SettingValue: "Gitea User Setting Test"}

	// create setting
	err := SetSetting(newSetting)
	assert.NoError(t, err)

	// get specific setting
	Settings, err := GetSetting(99, []string{keyName})
	assert.NoError(t, err)
	assert.Len(t, Settings, 1)
	assert.EqualValues(t, newSetting.SettingValue, Settings[keyName].SettingValue)

	// updated setting
	updatedSetting := &Setting{UserID: 99, SettingKey: keyName, SettingValue: "Updated", ID: Settings[keyName].ID}
	err = SetSetting(updatedSetting)
	assert.NoError(t, err)

	// get all settings
	Settings, err = GetUserAllSettings(99)
	assert.NoError(t, err)
	assert.Len(t, Settings, 1)
	assert.EqualValues(t, Settings[updatedSetting.SettingKey].SettingValue, updatedSetting.SettingValue)

	// delete setting
	err = DeleteSetting(updatedSetting)
	assert.NoError(t, err)
}
