// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getStorageInheritNameSectionTypeForLFS(t *testing.T) {
	iniStr := `
	[storage]
	STORAGE_TYPE = minio
	`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadLFSFrom(cfg))

	assert.EqualValues(t, "minio", LFS.Storage.Type)
	assert.EqualValues(t, "lfs/", cfg.Section("lfs").Key("MINIO_BASE_PATH").MustString(""))

	iniStr = `
[storage.lfs]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadLFSFrom(cfg))

	assert.EqualValues(t, "minio", LFS.Storage.Type)
	assert.EqualValues(t, "lfs/", cfg.Section("lfs").Key("MINIO_BASE_PATH").MustString(""))

	iniStr = `
[lfs]
STORAGE_TYPE = my_minio

[storage.my_minio]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadLFSFrom(cfg))

	assert.EqualValues(t, "minio", LFS.Storage.Type)
	assert.EqualValues(t, "lfs/", cfg.Section("lfs").Key("MINIO_BASE_PATH").MustString(""))

	iniStr = `
[lfs]
STORAGE_TYPE = my_minio
MINIO_BASE_PATH = my_lfs/

[storage.my_minio]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadLFSFrom(cfg))

	assert.EqualValues(t, "minio", LFS.Storage.Type)
	assert.EqualValues(t, "my_lfs/", cfg.Section("lfs").Key("MINIO_BASE_PATH").MustString(""))
}
