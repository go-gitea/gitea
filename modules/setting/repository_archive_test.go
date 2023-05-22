// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getStorageInheritNameSectionTypeForRepoArchive(t *testing.T) {
	// packages storage inherits from storage if nothing configured
	iniStr := `
[storage]
STORAGE_TYPE = minio
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadRepoArchiveFrom(cfg))

	assert.EqualValues(t, "minio", RepoArchive.Storage.Type)
	assert.EqualValues(t, "repo-archive/", cfg.Section("repo-archive").Key("MINIO_BASE_PATH").MustString(""))

	// we can also configure packages storage directly
	iniStr = `
[storage.repo-archive]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadRepoArchiveFrom(cfg))

	assert.EqualValues(t, "minio", RepoArchive.Storage.Type)
	assert.EqualValues(t, "repo-archive/", cfg.Section("repo-archive").Key("MINIO_BASE_PATH").MustString(""))

	// or we can indicate the storage type in the packages section
	iniStr = `
[repo-archive]
STORAGE_TYPE = my_minio

[storage.my_minio]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadRepoArchiveFrom(cfg))

	assert.EqualValues(t, "minio", RepoArchive.Storage.Type)
	assert.EqualValues(t, "repo-archive/", cfg.Section("repo-archive").Key("MINIO_BASE_PATH").MustString(""))

	// or we can indicate the storage type  and minio base path in the packages section
	iniStr = `
[repo-archive]
STORAGE_TYPE = my_minio
MINIO_BASE_PATH = my_archive/

[storage.my_minio]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadRepoArchiveFrom(cfg))

	assert.EqualValues(t, "minio", RepoArchive.Storage.Type)
	assert.EqualValues(t, "my_archive/", cfg.Section("repo-archive").Key("MINIO_BASE_PATH").MustString(""))
}
