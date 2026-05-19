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
STORAGE_TYPE = minio
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
STORAGE_TYPE = minio
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
STORAGE_TYPE = minio
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
	testLocalStoragePath(t, "/appdata", `
[storage]
STORAGE_TYPE = local
`, []testLocalStoragePathCase{
		{loadAttachmentFrom, &Attachment.Storage, "/appdata/attachments"},
		{loadLFSFrom, &LFS.Storage, "/appdata/lfs"},
		{loadActionsFrom, &Actions.ArtifactStorage, "/appdata/actions_artifacts"},
		{loadPackagesFrom, &Packages.Storage, "/appdata/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/appdata/repo-archive"},
		{loadActionsFrom, &Actions.LogStorage, "/appdata/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/appdata/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/appdata/repo-avatars"},
	})
}

func Test_getStorageInheritStorageTypeLocalPath(t *testing.T) {
	testLocalStoragePath(t, "/appdata", `
[storage]
STORAGE_TYPE = local
PATH = /data/gitea
`, []testLocalStoragePathCase{
		{loadAttachmentFrom, &Attachment.Storage, "/data/gitea/attachments"},
		{loadLFSFrom, &LFS.Storage, "/data/gitea/lfs"},
		{loadActionsFrom, &Actions.ArtifactStorage, "/data/gitea/actions_artifacts"},
		{loadPackagesFrom, &Packages.Storage, "/data/gitea/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/data/gitea/repo-archive"},
		{loadActionsFrom, &Actions.LogStorage, "/data/gitea/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/data/gitea/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/data/gitea/repo-avatars"},
	})
}

func Test_getStorageInheritStorageTypeLocalRelativePath(t *testing.T) {
	testLocalStoragePath(t, "/appdata", `
[storage]
STORAGE_TYPE = local
PATH = storages
`, []testLocalStoragePathCase{
		{loadAttachmentFrom, &Attachment.Storage, "/appdata/storages/attachments"},
		{loadLFSFrom, &LFS.Storage, "/appdata/storages/lfs"},
		{loadActionsFrom, &Actions.ArtifactStorage, "/appdata/storages/actions_artifacts"},
		{loadPackagesFrom, &Packages.Storage, "/appdata/storages/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/appdata/storages/repo-archive"},
		{loadActionsFrom, &Actions.LogStorage, "/appdata/storages/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/appdata/storages/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/appdata/storages/repo-avatars"},
	})
}

func Test_getStorageInheritStorageTypeLocalPathOverride(t *testing.T) {
	testLocalStoragePath(t, "/appdata", `
[storage]
STORAGE_TYPE = local
PATH = /data/gitea

[repo-archive]
PATH = /data/gitea/the-archives-dir
`, []testLocalStoragePathCase{
		{loadAttachmentFrom, &Attachment.Storage, "/data/gitea/attachments"},
		{loadLFSFrom, &LFS.Storage, "/data/gitea/lfs"},
		{loadActionsFrom, &Actions.ArtifactStorage, "/data/gitea/actions_artifacts"},
		{loadPackagesFrom, &Packages.Storage, "/data/gitea/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/data/gitea/the-archives-dir"},
		{loadActionsFrom, &Actions.LogStorage, "/data/gitea/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/data/gitea/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/data/gitea/repo-avatars"},
	})
}

func Test_getStorageInheritStorageTypeLocalPathOverrideEmpty(t *testing.T) {
	testLocalStoragePath(t, "/appdata", `
[storage]
STORAGE_TYPE = local
PATH = /data/gitea

[repo-archive]
`, []testLocalStoragePathCase{
		{loadAttachmentFrom, &Attachment.Storage, "/data/gitea/attachments"},
		{loadLFSFrom, &LFS.Storage, "/data/gitea/lfs"},
		{loadActionsFrom, &Actions.ArtifactStorage, "/data/gitea/actions_artifacts"},
		{loadPackagesFrom, &Packages.Storage, "/data/gitea/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/data/gitea/repo-archive"},
		{loadActionsFrom, &Actions.LogStorage, "/data/gitea/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/data/gitea/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/data/gitea/repo-avatars"},
	})
}

func Test_getStorageInheritStorageTypeLocalRelativePathOverride(t *testing.T) {
	testLocalStoragePath(t, "/appdata", `
[storage]
STORAGE_TYPE = local
PATH = /data/gitea

[repo-archive]
PATH = the-archives-dir
`, []testLocalStoragePathCase{
		{loadAttachmentFrom, &Attachment.Storage, "/data/gitea/attachments"},
		{loadLFSFrom, &LFS.Storage, "/data/gitea/lfs"},
		{loadActionsFrom, &Actions.ArtifactStorage, "/data/gitea/actions_artifacts"},
		{loadPackagesFrom, &Packages.Storage, "/data/gitea/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/data/gitea/the-archives-dir"},
		{loadActionsFrom, &Actions.LogStorage, "/data/gitea/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/data/gitea/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/data/gitea/repo-avatars"},
	})
}

func Test_getStorageInheritStorageTypeLocalPathOverride3(t *testing.T) {
	testLocalStoragePath(t, "/appdata", `
[storage.repo-archive]
STORAGE_TYPE = local
PATH = /data/gitea/archives
`, []testLocalStoragePathCase{
		{loadAttachmentFrom, &Attachment.Storage, "/appdata/attachments"},
		{loadLFSFrom, &LFS.Storage, "/appdata/lfs"},
		{loadActionsFrom, &Actions.ArtifactStorage, "/appdata/actions_artifacts"},
		{loadPackagesFrom, &Packages.Storage, "/appdata/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/data/gitea/archives"},
		{loadActionsFrom, &Actions.LogStorage, "/appdata/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/appdata/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/appdata/repo-avatars"},
	})
}

func Test_getStorageInheritStorageTypeLocalPathOverride3_5(t *testing.T) {
	testLocalStoragePath(t, "/appdata", `
[storage.repo-archive]
STORAGE_TYPE = local
PATH = a-relative-path
`, []testLocalStoragePathCase{
		{loadAttachmentFrom, &Attachment.Storage, "/appdata/attachments"},
		{loadLFSFrom, &LFS.Storage, "/appdata/lfs"},
		{loadActionsFrom, &Actions.ArtifactStorage, "/appdata/actions_artifacts"},
		{loadPackagesFrom, &Packages.Storage, "/appdata/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/appdata/a-relative-path"},
		{loadActionsFrom, &Actions.LogStorage, "/appdata/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/appdata/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/appdata/repo-avatars"},
	})
}

func Test_getStorageInheritStorageTypeLocalPathOverride4(t *testing.T) {
	testLocalStoragePath(t, "/appdata", `
[storage.repo-archive]
STORAGE_TYPE = local
PATH = /data/gitea/archives

[repo-archive]
PATH = /tmp/gitea/archives
`, []testLocalStoragePathCase{
		{loadAttachmentFrom, &Attachment.Storage, "/appdata/attachments"},
		{loadLFSFrom, &LFS.Storage, "/appdata/lfs"},
		{loadActionsFrom, &Actions.ArtifactStorage, "/appdata/actions_artifacts"},
		{loadPackagesFrom, &Packages.Storage, "/appdata/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/tmp/gitea/archives"},
		{loadActionsFrom, &Actions.LogStorage, "/appdata/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/appdata/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/appdata/repo-avatars"},
	})
}

func Test_getStorageInheritStorageTypeLocalPathOverride5(t *testing.T) {
	testLocalStoragePath(t, "/appdata", `
[storage.repo-archive]
STORAGE_TYPE = local
PATH = /data/gitea/archives

[repo-archive]
`, []testLocalStoragePathCase{
		{loadAttachmentFrom, &Attachment.Storage, "/appdata/attachments"},
		{loadLFSFrom, &LFS.Storage, "/appdata/lfs"},
		{loadActionsFrom, &Actions.ArtifactStorage, "/appdata/actions_artifacts"},
		{loadPackagesFrom, &Packages.Storage, "/appdata/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/data/gitea/archives"},
		{loadActionsFrom, &Actions.LogStorage, "/appdata/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/appdata/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/appdata/repo-avatars"},
	})
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
STORAGE_TYPE = minio
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
	cfg, err := NewConfigProviderFromData(`
[repo-archive]
STORAGE_TYPE = my_archive

[storage.my_archive]
; unsupported, storage type should be known type
STORAGE_TYPE = unknown // should be local or minio
PATH = archives
`)
	assert.NoError(t, err)
	assert.Error(t, loadRepoArchiveFrom(cfg))
}

func Test_getStorageConfiguration26(t *testing.T) {
	cfg, err := NewConfigProviderFromData(`
[repo-archive]
STORAGE_TYPE = minio
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
STORAGE_TYPE = minio
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
STORAGE_TYPE = minio
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
STORAGE_TYPE = minio
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
STORAGE_TYPE = minio
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
STORAGE_TYPE = minio
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
	resetStartupProblems := func(t *testing.T) {
		t.Helper()
		saved := StartupProblems
		StartupProblems = nil
		t.Cleanup(func() { StartupProblems = saved })
	}

	t.Run("legacy MINIO_ keys still populate config", func(t *testing.T) {
		resetStartupProblems(t)
		cfg, err := NewConfigProviderFromData(`
[storage]
STORAGE_TYPE = minio
MINIO_ENDPOINT = minio.example.com:9000
MINIO_ACCESS_KEY_ID = old_access
MINIO_SECRET_ACCESS_KEY = old_secret
MINIO_BUCKET = old-bucket
MINIO_BASE_PATH = /old-prefix
MINIO_USE_SSL = true
`)
		assert.NoError(t, err)
		assert.NoError(t, loadLFSFrom(cfg))
		assert.Equal(t, "minio.example.com:9000", LFS.Storage.S3Config.Endpoint)
		assert.Equal(t, "old_access", LFS.Storage.S3Config.AccessKeyID)
		assert.Equal(t, "old_secret", LFS.Storage.S3Config.SecretAccessKey)
		assert.Equal(t, "old-bucket", LFS.Storage.S3Config.Bucket)
		assert.True(t, LFS.Storage.S3Config.UseSSL)
		assert.Equal(t, "/old-prefix/lfs/", LFS.Storage.S3Config.BasePath)

		joined := strings.Join(StartupProblems, "\n")
		assert.Contains(t, joined, "STORAGE_TYPE = minio")
		assert.Contains(t, joined, "MINIO_ENDPOINT")
		assert.Contains(t, joined, "MINIO_BUCKET")
		assert.Contains(t, joined, minioToS3RemovalVersion)
	})

	t.Run("S3_ takes precedence when both are set", func(t *testing.T) {
		resetStartupProblems(t)
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
}
