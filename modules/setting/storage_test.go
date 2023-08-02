// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getStorageMultipleName(t *testing.T) {
	iniStr := `
[lfs]
MINIO_BUCKET = gitea-lfs

[attachment]
MINIO_BUCKET = gitea-attachment

[storage]
STORAGE_TYPE = minio
MINIO_BUCKET = gitea-storage
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))
	assert.EqualValues(t, "gitea-attachment", Attachment.Storage.MinioConfig.Bucket)

	assert.NoError(t, loadLFSFrom(cfg))
	assert.EqualValues(t, "gitea-lfs", LFS.Storage.MinioConfig.Bucket)

	assert.NoError(t, loadAvatarsFrom(cfg))
	assert.EqualValues(t, "gitea-storage", Avatar.Storage.MinioConfig.Bucket)
}

func Test_getStorageUseOtherNameAsType(t *testing.T) {
	iniStr := `
[attachment]
STORAGE_TYPE = lfs

[storage.lfs]
STORAGE_TYPE = minio
MINIO_BUCKET = gitea-storage
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))
	assert.EqualValues(t, "gitea-storage", Attachment.Storage.MinioConfig.Bucket)

	assert.NoError(t, loadLFSFrom(cfg))
	assert.EqualValues(t, "gitea-storage", LFS.Storage.MinioConfig.Bucket)
}

func Test_getStorageInheritStorageType(t *testing.T) {
	iniStr := `
[storage]
STORAGE_TYPE = minio
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadPackagesFrom(cfg))
	assert.EqualValues(t, "minio", Packages.Storage.Type)
	assert.EqualValues(t, "gitea", Packages.Storage.MinioConfig.Bucket)
	assert.EqualValues(t, "packages/", Packages.Storage.MinioConfig.BasePath)

	assert.NoError(t, loadRepoArchiveFrom(cfg))
	assert.EqualValues(t, "minio", RepoArchive.Storage.Type)
	assert.EqualValues(t, "gitea", RepoArchive.Storage.MinioConfig.Bucket)
	assert.EqualValues(t, "repo-archive/", RepoArchive.Storage.MinioConfig.BasePath)

	assert.NoError(t, loadActionsFrom(cfg))
	assert.EqualValues(t, "minio", Actions.LogStorage.Type)
	assert.EqualValues(t, "gitea", Actions.LogStorage.MinioConfig.Bucket)
	assert.EqualValues(t, "actions_log/", Actions.LogStorage.MinioConfig.BasePath)

	assert.EqualValues(t, "minio", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "gitea", Actions.ArtifactStorage.MinioConfig.Bucket)
	assert.EqualValues(t, "actions_artifacts/", Actions.ArtifactStorage.MinioConfig.BasePath)

	assert.NoError(t, loadAvatarsFrom(cfg))
	assert.EqualValues(t, "minio", Avatar.Storage.Type)
	assert.EqualValues(t, "gitea", Avatar.Storage.MinioConfig.Bucket)
	assert.EqualValues(t, "avatars/", Avatar.Storage.MinioConfig.BasePath)

	assert.NoError(t, loadRepoAvatarFrom(cfg))
	assert.EqualValues(t, "minio", RepoAvatar.Storage.Type)
	assert.EqualValues(t, "gitea", RepoAvatar.Storage.MinioConfig.Bucket)
	assert.EqualValues(t, "repo-avatars/", RepoAvatar.Storage.MinioConfig.BasePath)
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
		expected, err := filepath.Abs(c.expectedPath)
		assert.NoError(t, err)
		actual, err := filepath.Abs(storage.Path)
		assert.NoError(t, err)
		assert.EqualValues(t, expected, actual)
	}
}

func Test_getStorageInheritStorageTypeLocal(t *testing.T) {
	testLocalStoragePath(t, "/tmp/data", `
[storage]
STORAGE_TYPE = local
`, []testLocalStoragePathCase{
		{loadPackagesFrom, &Packages.Storage, "/tmp/data/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/tmp/data/repo-archive"},
		{loadActionsFrom, &Actions.LogStorage, "/tmp/data/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/tmp/data/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/tmp/data/repo-avatars"},
	})
}

func Test_getStorageInheritStorageTypeLocalPath(t *testing.T) {
	testLocalStoragePath(t, "/tmp/data", `
[storage]
STORAGE_TYPE = local
PATH = /data/gitea
`, []testLocalStoragePathCase{
		{loadPackagesFrom, &Packages.Storage, "/data/gitea/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/data/gitea/repo-archive"},
		{loadActionsFrom, &Actions.LogStorage, "/data/gitea/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/data/gitea/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/data/gitea/repo-avatars"},
	})
}

func Test_getStorageInheritStorageTypeLocalRelativePath(t *testing.T) {
	testLocalStoragePath(t, "/tmp/data", `
[storage]
STORAGE_TYPE = local
PATH = storages
`, []testLocalStoragePathCase{
		{loadPackagesFrom, &Packages.Storage, "/tmp/data/storages/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/tmp/data/storages/repo-archive"},
		{loadActionsFrom, &Actions.LogStorage, "/tmp/data/storages/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/tmp/data/storages/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/tmp/data/storages/repo-avatars"},
	})
}

func Test_getStorageInheritStorageTypeLocalPathOverride(t *testing.T) {
	testLocalStoragePath(t, "/tmp/data", `
[storage]
STORAGE_TYPE = local
PATH = /data/gitea

[repo-archive]
PATH = /data/gitea/the-archives-dir
`, []testLocalStoragePathCase{
		{loadPackagesFrom, &Packages.Storage, "/data/gitea/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/data/gitea/the-archives-dir"},
		{loadActionsFrom, &Actions.LogStorage, "/data/gitea/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/data/gitea/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/data/gitea/repo-avatars"},
	})
}

func Test_getStorageInheritStorageTypeLocalPathOverrideEmpty(t *testing.T) {
	testLocalStoragePath(t, "/tmp/data", `
[storage]
STORAGE_TYPE = local
PATH = /data/gitea

[repo-archive]
`, []testLocalStoragePathCase{
		{loadPackagesFrom, &Packages.Storage, "/data/gitea/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/data/gitea/repo-archive"},
		{loadActionsFrom, &Actions.LogStorage, "/data/gitea/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/data/gitea/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/data/gitea/repo-avatars"},
	})
}

func Test_getStorageInheritStorageTypeLocalRelativePathOverride(t *testing.T) {
	testLocalStoragePath(t, "/tmp/data", `
[storage]
STORAGE_TYPE = local
PATH = /data/gitea

[repo-archive]
PATH = the-archives-dir
`, []testLocalStoragePathCase{
		{loadPackagesFrom, &Packages.Storage, "/data/gitea/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/tmp/data/the-archives-dir"},
		{loadActionsFrom, &Actions.LogStorage, "/data/gitea/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/data/gitea/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/data/gitea/repo-avatars"},
	})
}

func Test_getStorageInheritStorageTypeLocalPathOverride3(t *testing.T) {
	testLocalStoragePath(t, "/tmp/data", `
[storage.repo-archive]
STORAGE_TYPE = local
PATH = /data/gitea/archives
`, []testLocalStoragePathCase{
		{loadPackagesFrom, &Packages.Storage, "/tmp/data/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/data/gitea/archives"},
		{loadActionsFrom, &Actions.LogStorage, "/tmp/data/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/tmp/data/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/tmp/data/repo-avatars"},
	})
}

func Test_getStorageInheritStorageTypeLocalPathOverride4(t *testing.T) {
	testLocalStoragePath(t, "/tmp/data", `
[storage.repo-archive]
STORAGE_TYPE = local
PATH = /data/gitea/archives

[repo-archive]
PATH = /tmp/gitea/archives
`, []testLocalStoragePathCase{
		{loadPackagesFrom, &Packages.Storage, "/tmp/data/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/tmp/gitea/archives"},
		{loadActionsFrom, &Actions.LogStorage, "/tmp/data/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/tmp/data/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/tmp/data/repo-avatars"},
	})
}

func Test_getStorageInheritStorageTypeLocalPathOverride5(t *testing.T) {
	testLocalStoragePath(t, "/tmp/data", `
[storage.repo-archive]
STORAGE_TYPE = local
PATH = /data/gitea/archives

[repo-archive]
`, []testLocalStoragePathCase{
		{loadPackagesFrom, &Packages.Storage, "/tmp/data/packages"},
		{loadRepoArchiveFrom, &RepoArchive.Storage, "/data/gitea/archives"},
		{loadActionsFrom, &Actions.LogStorage, "/tmp/data/actions_log"},
		{loadAvatarsFrom, &Avatar.Storage, "/tmp/data/avatars"},
		{loadRepoAvatarFrom, &RepoAvatar.Storage, "/tmp/data/repo-avatars"},
	})
}
