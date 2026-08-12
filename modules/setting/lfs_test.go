// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func Test_getStorageInheritNameSectionTypeForLFS(t *testing.T) {
	defer test.MockVariableValue(&LFS)()

	localPath := func(want string) func(t *testing.T) {
		return func(t *testing.T) {
			assert.Equal(t, LocalStorageType, LFS.Storage.Type)
			assert.Contains(t, LFS.Storage.Path, want)
		}
	}

	testConfigLoad(t, []any{loadLFSFrom}, []configTestCase{
		{
			name: "inherits the global [storage] type",
			ini:  "[storage]\nSTORAGE_TYPE = minio",
			want: minioStorageAt("lfs", &LFS.Storage, "lfs/"),
		},
		{
			name:  "[lfs].PATH wins over the deprecated [server].LFS_CONTENT_PATH",
			ini:   "[server]\nLFS_CONTENT_PATH = path_ignored\n[lfs]\nPATH = path_used",
			want:  []configCheck{guard(&LFS.Storage)},
			check: localPath("path_used"),
		},
		{
			name:  "the deprecated [server].LFS_CONTENT_PATH is still honoured alone",
			ini:   "[server]\nLFS_CONTENT_PATH = deprecatedpath",
			want:  []configCheck{guard(&LFS.Storage)},
			check: localPath("deprecatedpath"),
		},
		{
			name: "[storage.lfs] configures lfs directly",
			ini:  "[storage.lfs]\nSTORAGE_TYPE = minio",
			want: minioStorageAt("lfs", &LFS.Storage, "lfs/"),
		},
		{
			name: "[lfs].STORAGE_TYPE can name another storage",
			ini:  "[lfs]\nSTORAGE_TYPE = my_minio\n\n[storage.my_minio]\nSTORAGE_TYPE = minio",
			want: minioStorageAt("lfs", &LFS.Storage, "lfs/"),
		},
		{
			name: "[lfs].MINIO_BASE_PATH overrides the named storage",
			ini:  "[lfs]\nSTORAGE_TYPE = my_minio\nMINIO_BASE_PATH = my_lfs/\n\n[storage.my_minio]\nSTORAGE_TYPE = minio",
			want: minioStorageAt("lfs", &LFS.Storage, "my_lfs/"),
		},
	})
}

func Test_LFSStorage1(t *testing.T) {
	defer test.MockVariableValue(&LFS)()

	testConfigLoad(t, []any{loadLFSFrom}, []configTestCase{
		{
			name: "the default bucket is inherited",
			ini:  "[storage]\nSTORAGE_TYPE = minio",
			want: minioStorage("lfs", &LFS.Storage, "gitea", "lfs/"),
		},
	})
}

func Test_LFSClientServerConfigs(t *testing.T) {
	defer test.MockVariableValue(&LFS)()
	defer test.MockVariableValue(&LFSClient)()

	testConfigLoad(t, []any{loadLFSFrom}, []configTestCase{
		{
			name: "a zero batch size falls back to the default",
			ini:  "[server]\nLFS_MAX_BATCH_SIZE = 100\n[lfs_client]\n# will default to 20\nBATCH_SIZE = 0",
			want: []configCheck{
				field("LFS_MAX_BATCH_SIZE", &LFS.MaxBatchSize, 100),
				field("BATCH_SIZE", &LFSClient.BatchSize, 20),
				field("BATCH_OPERATION_CONCURRENCY", &LFSClient.BatchOperationConcurrency, 8),
			},
		},
		{
			name: "explicit client values",
			ini:  "[lfs_client]\nBATCH_SIZE = 50\nBATCH_OPERATION_CONCURRENCY = 10",
			want: []configCheck{
				field("BATCH_SIZE", &LFSClient.BatchSize, 50),
				field("BATCH_OPERATION_CONCURRENCY", &LFSClient.BatchOperationConcurrency, 10),
			},
		},
	})
}
