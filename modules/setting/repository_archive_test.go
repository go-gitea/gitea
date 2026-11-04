// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getStorageInheritNameSectionTypeForRepoArchive(t *testing.T) {
	for _, tc := range []struct {
		ini, basePath string
	}{
		// packages storage inherits from storage if nothing configured
		{`
[storage]
STORAGE_TYPE = minio
`, "repo-archive/"},
		// we can also configure packages storage directly
		{`
[storage.repo-archive]
STORAGE_TYPE = minio
`, "repo-archive/"},
		// or we can indicate the storage type in the packages section
		{`
[repo-archive]
STORAGE_TYPE = my_minio

[storage.my_minio]
STORAGE_TYPE = minio
`, "repo-archive/"},
		// or we can indicate the storage type  and minio base path in the packages section
		{`
[repo-archive]
STORAGE_TYPE = my_minio
MINIO_BASE_PATH = my_archive/

[storage.my_minio]
STORAGE_TYPE = minio
`, "my_archive/"},
	} {
		cfg, err := NewConfigProviderFromData(tc.ini)
		assert.NoError(t, err)
		assert.NoError(t, loadRepoArchiveFrom(cfg))

		assert.EqualValues(t, "minio", RepoArchive.Storage.Type)
		assert.Equal(t, tc.basePath, RepoArchive.Storage.MinioConfig.BasePath)
	}
}

func Test_RepoArchiveStorage(t *testing.T) {
	for _, ini := range []string{
		`
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
[storage]
STORAGE_TYPE            = minio
MINIO_ENDPOINT          = s3.my-domain.net
MINIO_BUCKET            = gitea
MINIO_LOCATION          = homenet
MINIO_USE_SSL           = true
MINIO_ACCESS_KEY_ID     = correct_key
MINIO_SECRET_ACCESS_KEY = correct_key
`,
		`
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
[storage.repo-archive]
STORAGE_TYPE = s3
[storage.s3]
STORAGE_TYPE            = minio
MINIO_ENDPOINT          = s3.my-domain.net
MINIO_BUCKET            = gitea
MINIO_LOCATION          = homenet
MINIO_USE_SSL           = true
MINIO_ACCESS_KEY_ID     = correct_key
MINIO_SECRET_ACCESS_KEY = correct_key
`,
	} {
		cfg, err := NewConfigProviderFromData(ini)
		assert.NoError(t, err)

		assert.NoError(t, loadRepoArchiveFrom(cfg))
		storage := RepoArchive.Storage

		assert.EqualValues(t, "minio", storage.Type)
		assert.Equal(t, "gitea", storage.MinioConfig.Bucket)
	}
}
