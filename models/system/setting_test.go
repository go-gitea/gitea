// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package system_test

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestSettings(t *testing.T) {
	keyName := "server.LFS_LOCKS_PAGING_NUM"
	assert.NoError(t, unittest.PrepareTestDatabase())

	newSetting := &system.Setting{SettingKey: keyName, SettingValue: "50"}

	// create setting
	err := system.SetSetting(newSetting)
	assert.NoError(t, err)
	// test about saving unchanged values
	err = system.SetSetting(newSetting)
	assert.NoError(t, err)

	// get specific setting
	settings, err := system.GetSettings([]string{keyName})
	assert.NoError(t, err)
	assert.Len(t, settings, 1)
	assert.EqualValues(t, newSetting.SettingValue, settings[strings.ToLower(keyName)].SettingValue)

	// updated setting
	updatedSetting := &system.Setting{SettingKey: keyName, SettingValue: "100", Version: newSetting.Version}
	err = system.SetSetting(updatedSetting)
	assert.NoError(t, err)

	// get all settings
	settings, err = system.GetAllSettings()
	assert.NoError(t, err)
	assert.Len(t, settings, 3)
	assert.EqualValues(t, updatedSetting.SettingValue, settings[strings.ToLower(updatedSetting.SettingKey)].SettingValue)

	// delete setting
	err = system.DeleteSetting(&system.Setting{SettingKey: strings.ToLower(keyName)})
	assert.NoError(t, err)
	settings, err = system.GetAllSettings()
	assert.NoError(t, err)
	assert.Len(t, settings, 2)
}
