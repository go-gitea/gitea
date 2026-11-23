// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getStorageMultipleName(t *testing.T) {
	iniStr := `
[lfs]
S3_BUCKET = gitea-lfs

[attachment]
S3_BUCKET = gitea-attachment

[storage]
STORAGE_TYPE = s3
S3_BUCKET = gitea-storage
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))
	assert.Equal(t, "gitea-attachment", Attachment.Storage.S3Config.Bucket)
	assert.Equal(t, "attachments/", Attachment.Storage.S3Config.BasePath)

	assert.NoError(t, loadLFSFrom(cfg))
	assert.Equal(t, "gitea-lfs", LFS.Storage.S3Config.Bucket)
	assert.Equal(t, "lfs/", LFS.Storage.S3Config.BasePath)

	assert.NoError(t, loadAvatarsFrom(cfg))
	assert.Equal(t, "gitea-storage", Avatar.Storage.S3Config.Bucket)
	assert.Equal(t, "avatars/", Avatar.Storage.S3Config.BasePath)
}

func Test_getStorageUseOtherNameAsType(t *testing.T) {
	iniStr := `
[attachment]
STORAGE_TYPE = lfs

[storage.lfs]
STORAGE_TYPE = s3
S3_BUCKET = gitea-storage
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))
	assert.Equal(t, "gitea-storage", Attachment.Storage.S3Config.Bucket)
	assert.Equal(t, "attachments/", Attachment.Storage.S3Config.BasePath)

	assert.NoError(t, loadLFSFrom(cfg))
	assert.Equal(t, "gitea-storage", LFS.Storage.S3Config.Bucket)
	assert.Equal(t, "lfs/", LFS.Storage.S3Config.BasePath)
}

func Test_getStorageInheritStorageType(t *testing.T) {
	iniStr := `
[storage]
STORAGE_TYPE = s3
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadPackagesFrom(cfg))
	assert.EqualValues(t, "s3", Packages.Storage.Type)
	assert.Equal(t, "gitea", Packages.Storage.S3Config.Bucket)
	assert.Equal(t, "packages/", Packages.Storage.S3Config.BasePath)

	assert.NoError(t, loadRepoArchiveFrom(cfg))
	assert.EqualValues(t, "s3", RepoArchive.Storage.Type)
	assert.Equal(t, "gitea", RepoArchive.Storage.S3Config.Bucket)
	assert.Equal(t, "repo-archive/", RepoArchive.Storage.S3Config.BasePath)

	assert.NoError(t, loadActionsFrom(cfg))
	assert.EqualValues(t, "s3", Actions.LogStorage.Type)
	assert.Equal(t, "gitea", Actions.LogStorage.S3Config.Bucket)
	assert.Equal(t, "actions_log/", Actions.LogStorage.S3Config.BasePath)

	assert.EqualValues(t, "s3", Actions.ArtifactStorage.Type)
	assert.Equal(t, "gitea", Actions.ArtifactStorage.S3Config.Bucket)
	assert.Equal(t, "actions_artifacts/", Actions.ArtifactStorage.S3Config.BasePath)

	assert.NoError(t, loadAvatarsFrom(cfg))
	assert.EqualValues(t, "s3", Avatar.Storage.Type)
	assert.Equal(t, "gitea", Avatar.Storage.S3Config.Bucket)
	assert.Equal(t, "avatars/", Avatar.Storage.S3Config.BasePath)

	assert.NoError(t, loadRepoAvatarFrom(cfg))
	assert.EqualValues(t, "s3", RepoAvatar.Storage.Type)
	assert.Equal(t, "gitea", RepoAvatar.Storage.S3Config.Bucket)
	assert.Equal(t, "repo-avatars/", RepoAvatar.Storage.S3Config.BasePath)
}

func Test_getStorageInheritStorageTypeAzureBlob(t *testing.T) {
	iniStr := `
[storage]
STORAGE_TYPE = azureblob
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadPackagesFrom(cfg))
	assert.EqualValues(t, "azureblob", Packages.Storage.Type)
	assert.Equal(t, "gitea", Packages.Storage.AzureBlobConfig.Container)
	assert.Equal(t, "packages/", Packages.Storage.AzureBlobConfig.BasePath)

	assert.NoError(t, loadRepoArchiveFrom(cfg))
	assert.EqualValues(t, "azureblob", RepoArchive.Storage.Type)
	assert.Equal(t, "gitea", RepoArchive.Storage.AzureBlobConfig.Container)
	assert.Equal(t, "repo-archive/", RepoArchive.Storage.AzureBlobConfig.BasePath)

	assert.NoError(t, loadActionsFrom(cfg))
	assert.EqualValues(t, "azureblob", Actions.LogStorage.Type)
	assert.Equal(t, "gitea", Actions.LogStorage.AzureBlobConfig.Container)
	assert.Equal(t, "actions_log/", Actions.LogStorage.AzureBlobConfig.BasePath)

	assert.EqualValues(t, "azureblob", Actions.ArtifactStorage.Type)
	assert.Equal(t, "gitea", Actions.ArtifactStorage.AzureBlobConfig.Container)
	assert.Equal(t, "actions_artifacts/", Actions.ArtifactStorage.AzureBlobConfig.BasePath)

	assert.NoError(t, loadAvatarsFrom(cfg))
	assert.EqualValues(t, "azureblob", Avatar.Storage.Type)
	assert.Equal(t, "gitea", Avatar.Storage.AzureBlobConfig.Container)
	assert.Equal(t, "avatars/", Avatar.Storage.AzureBlobConfig.BasePath)

	assert.NoError(t, loadRepoAvatarFrom(cfg))
	assert.EqualValues(t, "azureblob", RepoAvatar.Storage.Type)
	assert.Equal(t, "gitea", RepoAvatar.Storage.AzureBlobConfig.Container)
	assert.Equal(t, "repo-avatars/", RepoAvatar.Storage.AzureBlobConfig.BasePath)
}

type testLocalStoragePathCase struct {
	loader       func(rootCfg ConfigProvider) error
	storagePtr   **Storage
	expectedPath string
}

func testLocalStoragePath(t *testing.T, appDataPath, iniStr string, cases []testLocalStoragePathCase) {
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	AppDataPath = appDataPath
	for _, c := range cases {
		assert.NoError(t, c.loader(cfg))
		storage := *c.storagePtr

		assert.EqualValues(t, "local", storage.Type)
		assert.True(t, filepath.IsAbs(storage.Path))
		assert.Equal(t, filepath.Clean(c.expectedPath), filepath.Clean(storage.Path))
	}
}

func Test_getStorageInheritStorageTypeLocal(t *testing.T) {
	for _, tc := range []struct {
		name, ini, basePath, archivePath string
	}{
		{"Default", `
[storage]
STORAGE_TYPE = local
`, "/appdata", "/appdata/repo-archive"},
		{"Path", `
[storage]
STORAGE_TYPE = local
PATH = /data/gitea
`, "/data/gitea", "/data/gitea/repo-archive"},
		{"RelativePath", `
[storage]
STORAGE_TYPE = local
PATH = storages
`, "/appdata/storages", "/appdata/storages/repo-archive"},
		{"PathOverride", `
[storage]
STORAGE_TYPE = local
PATH = /data/gitea

[repo-archive]
PATH = /data/gitea/the-archives-dir
`, "/data/gitea", "/data/gitea/the-archives-dir"},
		{"PathOverrideEmpty", `
[storage]
STORAGE_TYPE = local
PATH = /data/gitea

[repo-archive]
`, "/data/gitea", "/data/gitea/repo-archive"},
		{"RelativePathOverride", `
[storage]
STORAGE_TYPE = local
PATH = /data/gitea

[repo-archive]
PATH = the-archives-dir
`, "/data/gitea", "/data/gitea/the-archives-dir"},
		{"PathOverride3", `
[storage.repo-archive]
STORAGE_TYPE = local
PATH = /data/gitea/archives
`, "/appdata", "/data/gitea/archives"},
		{"PathOverride3_5", `
[storage.repo-archive]
STORAGE_TYPE = local
PATH = a-relative-path
`, "/appdata", "/appdata/a-relative-path"},
		{"PathOverride4", `
[storage.repo-archive]
STORAGE_TYPE = local
PATH = /data/gitea/archives

[repo-archive]
PATH = /tmp/gitea/archives
`, "/appdata", "/tmp/gitea/archives"},
		{"PathOverride5", `
[storage.repo-archive]
STORAGE_TYPE = local
PATH = /data/gitea/archives

[repo-archive]
`, "/appdata", "/data/gitea/archives"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testLocalStoragePath(t, "/appdata", tc.ini, []testLocalStoragePathCase{
				{loadAttachmentFrom, &Attachment.Storage, tc.basePath + "/attachments"},
				{loadLFSFrom, &LFS.Storage, tc.basePath + "/lfs"},
				{loadActionsFrom, &Actions.ArtifactStorage, tc.basePath + "/actions_artifacts"},
				{loadPackagesFrom, &Packages.Storage, tc.basePath + "/packages"},
				{loadRepoArchiveFrom, &RepoArchive.Storage, tc.archivePath},
				{loadActionsFrom, &Actions.LogStorage, tc.basePath + "/actions_log"},
				{loadAvatarsFrom, &Avatar.Storage, tc.basePath + "/avatars"},
				{loadRepoAvatarFrom, &RepoAvatar.Storage, tc.basePath + "/repo-avatars"},
			})
		})
	}
}

func Test_getStorageInheritStorageTypeLocalPathOverride72(t *testing.T) {
	testLocalStoragePath(t, "/appdata", `
[repo-archive]
STORAGE_TYPE = local
PATH = archives
`, []testLocalStoragePathCase{
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/appdata/archives"},
	})
}

func Test_getStorageConfiguration20(t *testing.T) {
	cfg, err := NewConfigProviderFromData(`
[repo-archive]
STORAGE_TYPE = my_storage
PATH = archives
`)
	assert.NoError(t, err)

	assert.Error(t, loadRepoArchiveFrom(cfg))
}

func Test_getStorageConfiguration21(t *testing.T) {
	testLocalStoragePath(t, "/appdata", `
[storage.repo-archive]
`, []testLocalStoragePathCase{
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/appdata/repo-archive"},
	})
}

func Test_getStorageConfiguration22(t *testing.T) {
	testLocalStoragePath(t, "/appdata", `
[storage.repo-archive]
PATH = archives
`, []testLocalStoragePathCase{
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/appdata/archives"},
	})
}

func Test_getStorageConfiguration23(t *testing.T) {
	cfg, err := NewConfigProviderFromData(`
[repo-archive]
STORAGE_TYPE = s3
S3_ACCESS_KEY_ID = my_access_key
S3_SECRET_ACCESS_KEY = my_secret_key
`)
	assert.NoError(t, err)

	_, err = getStorage(cfg, "", "", nil)
	assert.Error(t, err)

	assert.NoError(t, loadRepoArchiveFrom(cfg))
	cp := RepoArchive.Storage.ToShadowCopy()
	assert.Equal(t, "******", cp.S3Config.AccessKeyID)
	assert.Equal(t, "******", cp.S3Config.SecretAccessKey)
}

func Test_getStorageConfiguration24(t *testing.T) {
	cfg, err := NewConfigProviderFromData(`
[repo-archive]
STORAGE_TYPE = my_archive

[storage.my_archive]
; unsupported, storage type should be defined explicitly
PATH = archives
`)
	assert.NoError(t, err)
	assert.Error(t, loadRepoArchiveFrom(cfg))
}

func Test_getStorageConfiguration25(t *testing.T) {
	for _, storageType := range []string{"my_archive", "local"} {
		cfg, err := NewConfigProviderFromData(`
[repo-archive]
STORAGE_TYPE = ` + storageType + `

[storage.` + storageType + `]
STORAGE_TYPE = unknown
PATH = archives
`)
		assert.NoError(t, err)
		assert.Error(t, loadRepoArchiveFrom(cfg), storageType)
	}
}

func Test_getStorageConfiguration26(t *testing.T) {
	cfg, err := NewConfigProviderFromData(`
[repo-archive]
STORAGE_TYPE = s3
S3_ACCESS_KEY_ID = my_access_key
S3_SECRET_ACCESS_KEY = my_secret_key
; wrong configuration
S3_USE_SSL = abc
`)
	assert.NoError(t, err)
	// assert.Error(t, loadRepoArchiveFrom(cfg))
	// FIXME: this should return error but now ini package's MapTo() doesn't check type
	assert.NoError(t, loadRepoArchiveFrom(cfg))
}

func Test_getStorageConfiguration27(t *testing.T) {
	cfg, err := NewConfigProviderFromData(`
[storage.repo-archive]
STORAGE_TYPE = s3
S3_ACCESS_KEY_ID = my_access_key
S3_SECRET_ACCESS_KEY = my_secret_key
S3_USE_SSL = true
`)
	assert.NoError(t, err)
	assert.NoError(t, loadRepoArchiveFrom(cfg))
	assert.Equal(t, "my_access_key", RepoArchive.Storage.S3Config.AccessKeyID)
	assert.Equal(t, "my_secret_key", RepoArchive.Storage.S3Config.SecretAccessKey)
	assert.True(t, RepoArchive.Storage.S3Config.UseSSL)
	assert.Equal(t, "repo-archive/", RepoArchive.Storage.S3Config.BasePath)
}

func Test_getStorageConfiguration28(t *testing.T) {
	cfg, err := NewConfigProviderFromData(`
[storage]
STORAGE_TYPE = s3
S3_ACCESS_KEY_ID = my_access_key
S3_SECRET_ACCESS_KEY = my_secret_key
S3_USE_SSL = true
S3_BASE_PATH = /prefix
`)
	assert.NoError(t, err)
	assert.NoError(t, loadRepoArchiveFrom(cfg))
	assert.Equal(t, "my_access_key", RepoArchive.Storage.S3Config.AccessKeyID)
	assert.Equal(t, "my_secret_key", RepoArchive.Storage.S3Config.SecretAccessKey)
	assert.True(t, RepoArchive.Storage.S3Config.UseSSL)
	assert.Equal(t, "/prefix/repo-archive/", RepoArchive.Storage.S3Config.BasePath)

	cfg, err = NewConfigProviderFromData(`
[storage]
STORAGE_TYPE = s3
S3_IAM_ENDPOINT = 127.0.0.1
S3_USE_SSL = true
S3_BASE_PATH = /prefix
`)
	assert.NoError(t, err)
	assert.NoError(t, loadRepoArchiveFrom(cfg))
	assert.Equal(t, "127.0.0.1", RepoArchive.Storage.S3Config.IamEndpoint)
	assert.True(t, RepoArchive.Storage.S3Config.UseSSL)
	assert.Equal(t, "/prefix/repo-archive/", RepoArchive.Storage.S3Config.BasePath)

	cfg, err = NewConfigProviderFromData(`
[storage]
STORAGE_TYPE = s3
S3_ACCESS_KEY_ID = my_access_key
S3_SECRET_ACCESS_KEY = my_secret_key
S3_USE_SSL = true
S3_BASE_PATH = /prefix

[lfs]
S3_BASE_PATH = /lfs
`)
	assert.NoError(t, err)
	assert.NoError(t, loadLFSFrom(cfg))
	assert.Equal(t, "my_access_key", LFS.Storage.S3Config.AccessKeyID)
	assert.Equal(t, "my_secret_key", LFS.Storage.S3Config.SecretAccessKey)
	assert.True(t, LFS.Storage.S3Config.UseSSL)
	assert.Equal(t, "/lfs", LFS.Storage.S3Config.BasePath)

	cfg, err = NewConfigProviderFromData(`
[storage]
STORAGE_TYPE = s3
S3_ACCESS_KEY_ID = my_access_key
S3_SECRET_ACCESS_KEY = my_secret_key
S3_USE_SSL = true
S3_BASE_PATH = /prefix

[storage.lfs]
S3_BASE_PATH = /lfs
`)
	assert.NoError(t, err)
	assert.NoError(t, loadLFSFrom(cfg))
	assert.Equal(t, "my_access_key", LFS.Storage.S3Config.AccessKeyID)
	assert.Equal(t, "my_secret_key", LFS.Storage.S3Config.SecretAccessKey)
	assert.True(t, LFS.Storage.S3Config.UseSSL)
	assert.Equal(t, "/lfs", LFS.Storage.S3Config.BasePath)
}

func Test_getStorageConfiguration29(t *testing.T) {
	cfg, err := NewConfigProviderFromData(`
[repo-archive]
STORAGE_TYPE = azureblob
AZURE_BLOB_ACCOUNT_NAME = my_account_name
AZURE_BLOB_ACCOUNT_KEY = my_account_key
`)
	assert.NoError(t, err)
	// assert.Error(t, loadRepoArchiveFrom(cfg))
	// FIXME: this should return error but now ini package's MapTo() doesn't check type
	assert.NoError(t, loadRepoArchiveFrom(cfg))
}

func Test_getStorageConfiguration30(t *testing.T) {
	cfg, err := NewConfigProviderFromData(`
[storage.repo-archive]
STORAGE_TYPE = azureblob
AZURE_BLOB_ACCOUNT_NAME = my_account_name
AZURE_BLOB_ACCOUNT_KEY = my_account_key
`)
	assert.NoError(t, err)
	assert.NoError(t, loadRepoArchiveFrom(cfg))
	assert.Equal(t, "my_account_name", RepoArchive.Storage.AzureBlobConfig.AccountName)
	assert.Equal(t, "my_account_key", RepoArchive.Storage.AzureBlobConfig.AccountKey)
	assert.Equal(t, "repo-archive/", RepoArchive.Storage.AzureBlobConfig.BasePath)
}

func Test_getStorageConfiguration31(t *testing.T) {
	cfg, err := NewConfigProviderFromData(`
[storage]
STORAGE_TYPE = azureblob
AZURE_BLOB_ACCOUNT_NAME = my_account_name
AZURE_BLOB_ACCOUNT_KEY = my_account_key
AZURE_BLOB_BASE_PATH = /prefix
`)
	assert.NoError(t, err)
	assert.NoError(t, loadRepoArchiveFrom(cfg))
	assert.Equal(t, "my_account_name", RepoArchive.Storage.AzureBlobConfig.AccountName)
	assert.Equal(t, "my_account_key", RepoArchive.Storage.AzureBlobConfig.AccountKey)
	assert.Equal(t, "/prefix/repo-archive/", RepoArchive.Storage.AzureBlobConfig.BasePath)

	cfg, err = NewConfigProviderFromData(`
[storage]
STORAGE_TYPE = azureblob
AZURE_BLOB_ACCOUNT_NAME = my_account_name
AZURE_BLOB_ACCOUNT_KEY = my_account_key
AZURE_BLOB_BASE_PATH = /prefix

[lfs]
AZURE_BLOB_BASE_PATH = /lfs
`)
	assert.NoError(t, err)
	assert.NoError(t, loadLFSFrom(cfg))
	assert.Equal(t, "my_account_name", LFS.Storage.AzureBlobConfig.AccountName)
	assert.Equal(t, "my_account_key", LFS.Storage.AzureBlobConfig.AccountKey)
	assert.Equal(t, "/lfs", LFS.Storage.AzureBlobConfig.BasePath)

	cfg, err = NewConfigProviderFromData(`
[storage]
STORAGE_TYPE = azureblob
AZURE_BLOB_ACCOUNT_NAME = my_account_name
AZURE_BLOB_ACCOUNT_KEY = my_account_key
AZURE_BLOB_BASE_PATH = /prefix

[storage.lfs]
AZURE_BLOB_BASE_PATH = /lfs
`)
	assert.NoError(t, err)
	assert.NoError(t, loadLFSFrom(cfg))
	assert.Equal(t, "my_account_name", LFS.Storage.AzureBlobConfig.AccountName)
	assert.Equal(t, "my_account_key", LFS.Storage.AzureBlobConfig.AccountKey)
	assert.Equal(t, "/lfs", LFS.Storage.AzureBlobConfig.BasePath)
}

func Test_getStorageDeprecatedMinioKeys(t *testing.T) {
	t.Run("legacy MINIO_ keys still populate config", func(t *testing.T) {
		savedProblems := StartupProblems
		StartupProblems = nil
		defer func() { StartupProblems = savedProblems }()

		legacyKeys := []string{"MINIO_ENDPOINT", "MINIO_ACCESS_KEY_ID", "MINIO_SECRET_ACCESS_KEY", "MINIO_BUCKET", "MINIO_BASE_PATH", "MINIO_USE_SSL"}
		cfg, err := NewConfigProviderFromData(`
[storage]
STORAGE_TYPE = minio
MINIO_ENDPOINT = minio.example.com:9000
MINIO_IAM_ENDPOINT = https://%(MINIO_ENDPOINT)s/iam
MINIO_ACCESS_KEY_ID = old_access
MINIO_SECRET_ACCESS_KEY = old_secret
MINIO_BUCKET = old-bucket
MINIO_BASE_PATH = /old-prefix
MINIO_USE_SSL = true
`)
		assert.NoError(t, err)
		assert.NoError(t, loadLFSFrom(cfg))
		assert.Equal(t, "minio.example.com:9000", LFS.Storage.S3Config.Endpoint)
		assert.Equal(t, "https://minio.example.com:9000/iam", LFS.Storage.S3Config.IamEndpoint)
		assert.Equal(t, "old_access", LFS.Storage.S3Config.AccessKeyID)
		assert.Equal(t, "old_secret", LFS.Storage.S3Config.SecretAccessKey)
		assert.Equal(t, "old-bucket", LFS.Storage.S3Config.Bucket)
		assert.True(t, LFS.Storage.S3Config.UseSSL)
		assert.Equal(t, "/old-prefix/lfs/", LFS.Storage.S3Config.BasePath)

		joined := strings.Join(StartupProblems, "\n")
		assert.Contains(t, joined, "storage type `minio`")
		for _, k := range legacyKeys {
			assert.Contains(t, joined, k, "expected deprecation warning for %s", k)
		}
	})

	t.Run("S3_ takes precedence when both are set", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData(`
[storage]
STORAGE_TYPE = minio
MINIO_BUCKET = old-bucket
S3_BUCKET = new-bucket
`)
		assert.NoError(t, err)
		assert.NoError(t, loadLFSFrom(cfg))
		assert.Equal(t, "new-bucket", LFS.Storage.S3Config.Bucket)
	})

	t.Run("legacy overrides stay local after parent migration", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData(`
[storage]
STORAGE_TYPE = minio
MINIO_BUCKET = default-bucket
MINIO_BASE_PATH = /default-prefix

[storage.lfs]
MINIO_BUCKET = lfs-bucket
MINIO_BASE_PATH = /lfs-prefix
`)
		assert.NoError(t, err)
		assert.NoError(t, loadAttachmentFrom(cfg))
		assert.NoError(t, loadLFSFrom(cfg))
		assert.Equal(t, "lfs-bucket", LFS.Storage.S3Config.Bucket)
		assert.Equal(t, "/lfs-prefix", LFS.Storage.S3Config.BasePath)
		assert.NoError(t, loadAttachmentFrom(cfg))
		assert.Equal(t, "default-bucket", Attachment.Storage.S3Config.Bucket)
		assert.Equal(t, "/default-prefix/attachments/", Attachment.Storage.S3Config.BasePath)
	})

	t.Run("both types select the legacy section", func(t *testing.T) {
		for _, storageType := range []string{"s3", "minio"} {
			t.Run(storageType, func(t *testing.T) {
				cfg, err := NewConfigProviderFromData(`
[picture]
AVATAR_STORAGE_TYPE = ` + storageType + `

[storage]
MINIO_ACCESS_KEY_ID = inherited-access

[storage.lfs]
STORAGE_TYPE = ` + storageType + `

[storage.minio]
S3_ENDPOINT = s3.example.com:9000
S3_BUCKET = shared-bucket
`)
				assert.NoError(t, err)
				assert.NoError(t, loadLFSFrom(cfg))
				assert.Equal(t, S3StorageType, LFS.Storage.Type)
				assert.Equal(t, "inherited-access", LFS.Storage.S3Config.AccessKeyID)
				assert.Equal(t, "s3.example.com:9000", LFS.Storage.S3Config.Endpoint)
				assert.Equal(t, "shared-bucket", LFS.Storage.S3Config.Bucket)
				assert.NoError(t, loadAvatarsFrom(cfg))
				assert.Equal(t, S3StorageType, Avatar.Storage.Type)
				assert.Equal(t, "shared-bucket", Avatar.Storage.S3Config.Bucket)
			})
		}
	})

	t.Run("each type prefers its own section", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData(`
[picture]
AVATAR_STORAGE_TYPE = minio
REPOSITORY_AVATAR_STORAGE_TYPE = s3

[storage.minio]
S3_BUCKET = minio-bucket

[storage.s3]
S3_BUCKET = s3-bucket
`)
		assert.NoError(t, err)
		assert.NoError(t, loadAvatarsFrom(cfg))
		assert.NoError(t, loadRepoAvatarFrom(cfg))
		assert.Equal(t, "minio-bucket", Avatar.Storage.S3Config.Bucket)
		assert.Equal(t, "s3-bucket", RepoAvatar.Storage.S3Config.Bucket)
	})

	t.Run("legacy keys inherited from parent sections", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData(`
[actions]
STORAGE_TYPE = minio
MINIO_ENDPOINT = artifacts.example.com:9000

[actions.artifacts]
MINIO_BASE_PATH = /artifacts
`)
		assert.NoError(t, err)
		assert.NoError(t, loadActionsFrom(cfg))
		assert.Equal(t, S3StorageType, Actions.ArtifactStorage.Type)
		assert.Equal(t, "artifacts.example.com:9000", Actions.ArtifactStorage.S3Config.Endpoint)
		assert.Equal(t, "/artifacts", Actions.ArtifactStorage.S3Config.BasePath)
	})
}
