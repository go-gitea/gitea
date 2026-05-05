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
	assert.Equal(t, "repo-archive/", RepoArchive.Storage.MinioConfig.BasePath)

	// we can also configure packages storage directly
	iniStr = `
[storage.repo-archive]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadRepoArchiveFrom(cfg))

	assert.EqualValues(t, "minio", RepoArchive.Storage.Type)
	assert.Equal(t, "repo-archive/", RepoArchive.Storage.MinioConfig.BasePath)

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
	assert.Equal(t, "repo-archive/", RepoArchive.Storage.MinioConfig.BasePath)

	// or we can indicate the storage type  and minio base path in the packages section
	iniStr = `
[repo-archive]
STORAGE_TYPE = my_minio
S3_BASE_PATH = my_archive/

[storage.my_minio]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadRepoArchiveFrom(cfg))

	assert.EqualValues(t, "minio", RepoArchive.Storage.Type)
	assert.Equal(t, "my_archive/", RepoArchive.Storage.MinioConfig.BasePath)
}

func Test_RepoArchiveStorage(t *testing.T) {
	iniStr := `
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
[storage]
STORAGE_TYPE            = minio
S3_ENDPOINT = s3.my-domain.net
S3_BUCKET = gitea
S3_LOCATION = homenet
S3_USE_SSL = true
S3_ACCESS_KEY_ID = correct_key
S3_SECRET_ACCESS_KEY = correct_key
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadRepoArchiveFrom(cfg))
	storage := RepoArchive.Storage

	assert.EqualValues(t, "minio", storage.Type)
	assert.Equal(t, "gitea", storage.MinioConfig.Bucket)

	iniStr = `
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
[storage.repo-archive]
STORAGE_TYPE = s3
[storage.s3]
STORAGE_TYPE            = minio
S3_ENDPOINT = s3.my-domain.net
S3_BUCKET = gitea
S3_LOCATION = homenet
S3_USE_SSL = true
S3_ACCESS_KEY_ID = correct_key
S3_SECRET_ACCESS_KEY = correct_key
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadRepoArchiveFrom(cfg))
	storage = RepoArchive.Storage

	assert.EqualValues(t, "minio", storage.Type)
	assert.Equal(t, "gitea", storage.MinioConfig.Bucket)
}
