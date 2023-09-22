// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system_test

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/internal/models/db"
	"code.gitea.io/gitea/internal/models/system"
	"code.gitea.io/gitea/internal/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestSettings(t *testing.T) {
	keyName := "server.LFS_LOCKS_PAGING_NUM"
	assert.NoError(t, unittest.PrepareTestDatabase())

	newSetting := &system.Setting{SettingKey: keyName, SettingValue: "50"}

	// create setting
	err := system.SetSetting(db.DefaultContext, newSetting)
	assert.NoError(t, err)
	// test about saving unchanged values
	err = system.SetSetting(db.DefaultContext, newSetting)
	assert.NoError(t, err)

	// get specific setting
	settings, err := system.GetSettings(db.DefaultContext, []string{keyName})
	assert.NoError(t, err)
	assert.Len(t, settings, 1)
	assert.EqualValues(t, newSetting.SettingValue, settings[strings.ToLower(keyName)].SettingValue)

	// updated setting
	updatedSetting := &system.Setting{SettingKey: keyName, SettingValue: "100", Version: settings[strings.ToLower(keyName)].Version}
	err = system.SetSetting(db.DefaultContext, updatedSetting)
	assert.NoError(t, err)

	value, err := system.GetSetting(db.DefaultContext, keyName)
	assert.NoError(t, err)
	assert.EqualValues(t, updatedSetting.SettingValue, value.SettingValue)

	// get all settings
	settings, err = system.GetAllSettings(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, settings, 3)
	assert.EqualValues(t, updatedSetting.SettingValue, settings[strings.ToLower(updatedSetting.SettingKey)].SettingValue)

	// delete setting
	err = system.DeleteSetting(db.DefaultContext, &system.Setting{SettingKey: strings.ToLower(keyName)})
	assert.NoError(t, err)
	settings, err = system.GetAllSettings(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, settings, 2)
}
