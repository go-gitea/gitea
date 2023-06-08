// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
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
	assert.EqualValues(t, "actions_log/", Actions.LogStorage.MinioConfig.BasePath)
	assert.EqualValues(t, "minio", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "actions_artifacts/", Actions.ArtifactStorage.MinioConfig.BasePath)

	iniStr = `
[storage.actions_log]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadActionsFrom(cfg))

	assert.EqualValues(t, "minio", Actions.LogStorage.Type)
	assert.EqualValues(t, "actions_log/", Actions.LogStorage.MinioConfig.BasePath)
	assert.EqualValues(t, "local", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "actions_artifacts", filepath.Base(Actions.ArtifactStorage.Path))

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
	assert.EqualValues(t, "actions_log/", Actions.LogStorage.MinioConfig.BasePath)
	assert.EqualValues(t, "local", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "actions_artifacts", filepath.Base(Actions.ArtifactStorage.Path))

	iniStr = `
[storage.actions_artifacts]
STORAGE_TYPE = my_storage

[storage.my_storage]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadActionsFrom(cfg))

	assert.EqualValues(t, "local", Actions.LogStorage.Type)
	assert.EqualValues(t, "actions_log", filepath.Base(Actions.LogStorage.Path))
	assert.EqualValues(t, "minio", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "actions_artifacts/", Actions.ArtifactStorage.MinioConfig.BasePath)

	iniStr = `
[storage.actions_artifacts]
STORAGE_TYPE = my_storage

[storage.my_storage]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadActionsFrom(cfg))

	assert.EqualValues(t, "local", Actions.LogStorage.Type)
	assert.EqualValues(t, "actions_log", filepath.Base(Actions.LogStorage.Path))
	assert.EqualValues(t, "minio", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "actions_artifacts/", Actions.ArtifactStorage.MinioConfig.BasePath)

	iniStr = ``
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadActionsFrom(cfg))

	assert.EqualValues(t, "local", Actions.LogStorage.Type)
	assert.EqualValues(t, "actions_log", filepath.Base(Actions.LogStorage.Path))
	assert.EqualValues(t, "local", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "actions_artifacts", filepath.Base(Actions.ArtifactStorage.Path))
}
