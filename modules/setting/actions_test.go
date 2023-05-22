// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getStorageInheritNameSectionTypeForActions(t *testing.T) {
	iniStr := `
	[storage]
	STORAGE_TYPE = minio
	`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadActionsFrom(cfg))

	assert.EqualValues(t, "minio", Actions.LogStorage.Type)
	assert.EqualValues(t, "actions_log/", Actions.LogStorage.Section.Key("MINIO_BASE_PATH").String())

	iniStr = `
[storage.actions_log]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadActionsFrom(cfg))

	assert.EqualValues(t, "minio", Actions.LogStorage.Type)
	assert.EqualValues(t, "actions_log/", Actions.LogStorage.Section.Key("MINIO_BASE_PATH").String())

	iniStr = `
[storage.actions_log]
STORAGE_TYPE = my_storage

[storage.my_storage]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadActionsFrom(cfg))

	assert.EqualValues(t, "minio", Actions.LogStorage.Type)
	assert.EqualValues(t, "actions_log/", Actions.LogStorage.Section.Key("MINIO_BASE_PATH").String())
}
