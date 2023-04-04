// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	ini "gopkg.in/ini.v1"
)

func Test_getStorageInheritNameSectionTypeForActions(t *testing.T) {
	iniStr := `
	[storage]
	STORAGE_TYPE = minio
	`
	cfg, err := ini.Load([]byte(iniStr))
	assert.NoError(t, err)
	assert.NoError(t, loadActionsFrom(cfg))

	assert.EqualValues(t, "minio", Actions.Storage.Type)
	assert.EqualValues(t, "actions_log/", cfg.Section("actions").Key("MINIO_BASE_PATH").MustString(""))

	iniStr = `
[storage.actions_log]
STORAGE_TYPE = minio
`
	cfg, err = ini.Load([]byte(iniStr))
	assert.NoError(t, err)
	assert.NoError(t, loadActionsFrom(cfg))

	assert.EqualValues(t, "minio", Actions.Storage.Type)
	assert.EqualValues(t, "actions_log/", cfg.Section("actions").Key("MINIO_BASE_PATH").MustString(""))

	iniStr = `
[actions]
STORAGE_TYPE = my_minio

[storage.my_minio]
STORAGE_TYPE = minio
`
	cfg, err = ini.Load([]byte(iniStr))
	assert.NoError(t, err)
	assert.NoError(t, loadActionsFrom(cfg))

	assert.EqualValues(t, "minio", Actions.Storage.Type)
	assert.EqualValues(t, "actions_log/", cfg.Section("actions").Key("MINIO_BASE_PATH").MustString(""))

	iniStr = `
[actions]
STORAGE_TYPE = my_minio
MINIO_BASE_PATH = my_actions_log/

[storage.my_minio]
STORAGE_TYPE = minio
`
	cfg, err = ini.Load([]byte(iniStr))
	assert.NoError(t, err)
	assert.NoError(t, loadActionsFrom(cfg))

	assert.EqualValues(t, "minio", Actions.Storage.Type)
	assert.EqualValues(t, "my_actions_log/", cfg.Section("actions").Key("MINIO_BASE_PATH").MustString(""))
}
