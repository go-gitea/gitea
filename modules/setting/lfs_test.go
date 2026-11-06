// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func Test_getStorageInheritNameSectionTypeForLFS(t *testing.T) {
	for _, tt := range []struct {
		config, storageType, path string
	}{
		{`
	[storage]
	STORAGE_TYPE = minio
	`, "minio", "lfs/"},
		{`
[server]
LFS_CONTENT_PATH = path_ignored
[lfs]
PATH = path_used
`, "local", "path_used"},
		{`
[server]
LFS_CONTENT_PATH = deprecatedpath
`, "local", "deprecatedpath"},
		{`
[storage.lfs]
STORAGE_TYPE = minio
`, "minio", "lfs/"},
		{`
[lfs]
STORAGE_TYPE = my_minio

[storage.my_minio]
STORAGE_TYPE = minio
`, "minio", "lfs/"},
		{`
[lfs]
STORAGE_TYPE = my_minio
MINIO_BASE_PATH = my_lfs/

[storage.my_minio]
STORAGE_TYPE = minio
`, "minio", "my_lfs/"},
	} {
		cfg, err := NewConfigProviderFromData(tt.config)
		assert.NoError(t, err)
		if tt.storageType == "local" {
			applyDeprecatedKeys(cfg)
		}
		assert.NoError(t, loadLFSFrom(cfg))
		assert.EqualValues(t, tt.storageType, LFS.Storage.Type)
		if tt.storageType == "local" {
			assert.Contains(t, LFS.Storage.Path, tt.path)
		} else {
			assert.Equal(t, tt.path, LFS.Storage.MinioConfig.BasePath)
		}
	}
}

func Test_LFSStorage1(t *testing.T) {
	iniStr := `
[storage]
STORAGE_TYPE = minio
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadLFSFrom(cfg))
	assert.EqualValues(t, "minio", LFS.Storage.Type)
	assert.Equal(t, "gitea", LFS.Storage.MinioConfig.Bucket)
	assert.Equal(t, "lfs/", LFS.Storage.MinioConfig.BasePath)
}

func Test_LFSClientServerConfigs(t *testing.T) {
	defer test.MockVariableValue(&LFS)()
	defer test.MockVariableValue(&LFSClient)()

	iniStr := `
[server]
LFS_MAX_BATCH_SIZE = 100
[lfs_client]
# will default to 20
BATCH_SIZE = 0
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadLFSFrom(cfg))
	assert.Equal(t, 100, LFS.MaxBatchSize)
	assert.Equal(t, 20, LFSClient.BatchSize)
	assert.Equal(t, 8, LFSClient.BatchOperationConcurrency)

	iniStr = `
[lfs_client]
BATCH_SIZE = 50
BATCH_OPERATION_CONCURRENCY = 10
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadLFSFrom(cfg))
	assert.Equal(t, 50, LFSClient.BatchSize)
	assert.Equal(t, 10, LFSClient.BatchOperationConcurrency)
}
