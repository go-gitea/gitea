// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"
)

func Test_getStorageInheritNameSectionTypeForRepoArchive(t *testing.T) {
	defer test.MockVariableValue(&RepoArchive)()

	testConfigLoad(t, []any{loadRepoArchiveFrom}, []configTestCase{
		{
			name: "inherits from [storage] if nothing is configured",
			ini:  "[storage]\nSTORAGE_TYPE = minio",
			want: minioStorageAt("repo-archive", &RepoArchive.Storage, "repo-archive/"),
		},
		{
			name: "[storage.repo-archive] configures it directly",
			ini:  "[storage.repo-archive]\nSTORAGE_TYPE = minio",
			want: minioStorageAt("repo-archive", &RepoArchive.Storage, "repo-archive/"),
		},
		{
			name: "[repo-archive].STORAGE_TYPE can name another storage",
			ini:  "[repo-archive]\nSTORAGE_TYPE = my_minio\n\n[storage.my_minio]\nSTORAGE_TYPE = minio",
			want: minioStorageAt("repo-archive", &RepoArchive.Storage, "repo-archive/"),
		},
		{
			name: "[repo-archive].MINIO_BASE_PATH overrides the named storage",
			ini:  "[repo-archive]\nSTORAGE_TYPE = my_minio\nMINIO_BASE_PATH = my_archive/\n\n[storage.my_minio]\nSTORAGE_TYPE = minio",
			want: minioStorageAt("repo-archive", &RepoArchive.Storage, "my_archive/"),
		},
	})
}

func Test_RepoArchiveStorage(t *testing.T) {
	defer test.MockVariableValue(&RepoArchive)()

	minioSection := `
STORAGE_TYPE            = minio
MINIO_ENDPOINT          = s3.my-domain.net
MINIO_BUCKET            = gitea
MINIO_LOCATION          = homenet
MINIO_USE_SSL           = true
MINIO_ACCESS_KEY_ID     = correct_key
MINIO_SECRET_ACCESS_KEY = correct_key
`
	bucket := []configCheck{
		guard(&RepoArchive.Storage),
		fieldOf("STORAGE_TYPE", func() StorageType { return RepoArchive.Storage.Type }, MinioStorageType),
		fieldOf("MINIO_BUCKET", func() string { return RepoArchive.Storage.MinioConfig.Bucket }, "gitea"),
	}

	testConfigLoad(t, []any{loadRepoArchiveFrom}, []configTestCase{
		{
			name: "a fully configured [storage] section",
			ini:  ";;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;\n[storage]" + minioSection,
			want: bucket,
		},
		{
			name: "an indirection through a named storage",
			ini:  ";;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;\n[storage.repo-archive]\nSTORAGE_TYPE = s3\n[storage.s3]" + minioSection,
			want: bucket,
		},
	})
}
