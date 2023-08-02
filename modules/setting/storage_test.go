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

func Test_getStorageInheritStorageTypeLocal(t *testing.T) {
	iniStr := `
[storage]
STORAGE_TYPE = local
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadPackagesFrom(cfg))
	assert.EqualValues(t, "local", Packages.Storage.Type)
	assert.EqualValues(t, filepath.Join(AppDataPath, "packages"), Packages.Storage.Path)

	assert.NoError(t, loadRepoArchiveFrom(cfg))
	assert.EqualValues(t, "local", RepoArchive.Storage.Type)
	assert.EqualValues(t, filepath.Join(AppDataPath, "repo-archive"), RepoArchive.Storage.Path)

	assert.NoError(t, loadActionsFrom(cfg))
	assert.EqualValues(t, "local", Actions.LogStorage.Type)
	assert.EqualValues(t, filepath.Join(AppDataPath, "actions_log"), Actions.LogStorage.Path)

	assert.EqualValues(t, "local", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, filepath.Join(AppDataPath, "actions_artifacts"), Actions.ArtifactStorage.Path)

	assert.NoError(t, loadAvatarsFrom(cfg))
	assert.EqualValues(t, "local", Avatar.Storage.Type)
	assert.EqualValues(t, filepath.Join(AppDataPath, "avatars"), Avatar.Storage.Path)

	assert.NoError(t, loadRepoAvatarFrom(cfg))
	assert.EqualValues(t, "local", RepoAvatar.Storage.Type)
	assert.EqualValues(t, filepath.Join(AppDataPath, "repo-avatars"), RepoAvatar.Storage.Path)
}

func Test_getStorageInheritStorageTypeLocalDataPath(t *testing.T) {
	iniStr := `
[storage]
STORAGE_TYPE = local
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	AppDataPath = "/tmp/gitea"

	assert.NoError(t, loadPackagesFrom(cfg))
	assert.EqualValues(t, "local", Packages.Storage.Type)
	assert.EqualValues(t, "/tmp/gitea/packages", Packages.Storage.Path)

	assert.NoError(t, loadRepoArchiveFrom(cfg))
	assert.EqualValues(t, "local", RepoArchive.Storage.Type)
	assert.EqualValues(t, "/tmp/gitea/repo-archive", RepoArchive.Storage.Path)

	assert.NoError(t, loadActionsFrom(cfg))
	assert.EqualValues(t, "local", Actions.LogStorage.Type)
	assert.EqualValues(t, "/tmp/gitea/actions_log", Actions.LogStorage.Path)

	assert.EqualValues(t, "local", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "/tmp/gitea/actions_artifacts", Actions.ArtifactStorage.Path)

	assert.NoError(t, loadAvatarsFrom(cfg))
	assert.EqualValues(t, "local", Avatar.Storage.Type)
	assert.EqualValues(t, "/tmp/gitea/avatars", Avatar.Storage.Path)

	assert.NoError(t, loadRepoAvatarFrom(cfg))
	assert.EqualValues(t, "local", RepoAvatar.Storage.Type)
	assert.EqualValues(t, "/tmp/gitea/repo-avatars", RepoAvatar.Storage.Path)
}

func Test_getStorageInheritStorageTypeLocalPath(t *testing.T) {
	iniStr := `
[storage]
STORAGE_TYPE = local
PATH = /data/gitea
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadPackagesFrom(cfg))
	assert.EqualValues(t, "local", Packages.Storage.Type)
	assert.EqualValues(t, "/data/gitea/packages", Packages.Storage.Path)

	assert.NoError(t, loadRepoArchiveFrom(cfg))
	assert.EqualValues(t, "local", RepoArchive.Storage.Type)
	assert.EqualValues(t, "/data/gitea/repo-archive", RepoArchive.Storage.Path)

	assert.NoError(t, loadActionsFrom(cfg))
	assert.EqualValues(t, "local", Actions.LogStorage.Type)
	assert.EqualValues(t, "/data/gitea/actions_log", Actions.LogStorage.Path)

	assert.EqualValues(t, "local", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "/data/gitea/actions_artifacts", Actions.ArtifactStorage.Path)

	assert.NoError(t, loadAvatarsFrom(cfg))
	assert.EqualValues(t, "local", Avatar.Storage.Type)
	assert.EqualValues(t, "/data/gitea/avatars", Avatar.Storage.Path)

	assert.NoError(t, loadRepoAvatarFrom(cfg))
	assert.EqualValues(t, "local", RepoAvatar.Storage.Type)
	assert.EqualValues(t, "/data/gitea/repo-avatars", RepoAvatar.Storage.Path)
}

func Test_getStorageInheritStorageTypeLocalRelativePath(t *testing.T) {
	iniStr := `
[storage]
STORAGE_TYPE = local
PATH = storages
`
	AppDataPath = "/tmp/data"
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadPackagesFrom(cfg))
	assert.EqualValues(t, "local", Packages.Storage.Type)
	assert.EqualValues(t, filepath.Join(AppDataPath, "storages", "packages"), Packages.Storage.Path)

	assert.NoError(t, loadRepoArchiveFrom(cfg))
	assert.EqualValues(t, "local", RepoArchive.Storage.Type)
	assert.EqualValues(t, filepath.Join(AppDataPath, "storages", "repo-archive"), RepoArchive.Storage.Path)

	assert.NoError(t, loadActionsFrom(cfg))
	assert.EqualValues(t, "local", Actions.LogStorage.Type)
	assert.EqualValues(t, filepath.Join(AppDataPath, "storages", "actions_log"), Actions.LogStorage.Path)

	assert.EqualValues(t, "local", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, filepath.Join(AppDataPath, "storages", "actions_artifacts"), Actions.ArtifactStorage.Path)

	assert.NoError(t, loadAvatarsFrom(cfg))
	assert.EqualValues(t, "local", Avatar.Storage.Type)
	assert.EqualValues(t, filepath.Join(AppDataPath, "storages", "avatars"), Avatar.Storage.Path)

	assert.NoError(t, loadRepoAvatarFrom(cfg))
	assert.EqualValues(t, "local", RepoAvatar.Storage.Type)
	assert.EqualValues(t, filepath.Join(AppDataPath, "storages", "repo-avatars"), RepoAvatar.Storage.Path)
}

func Test_getStorageInheritStorageTypeLocalPathOverride(t *testing.T) {
	iniStr := `
[storage]
STORAGE_TYPE = local
PATH = /data/gitea

[repo-archive]
PATH = /data/gitea/archives
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadPackagesFrom(cfg))
	assert.EqualValues(t, "local", Packages.Storage.Type)
	assert.EqualValues(t, "/data/gitea/packages", Packages.Storage.Path)

	assert.NoError(t, loadRepoArchiveFrom(cfg))
	assert.EqualValues(t, "local", RepoArchive.Storage.Type)
	assert.EqualValues(t, "/data/gitea/archives", RepoArchive.Storage.Path)

	assert.NoError(t, loadActionsFrom(cfg))
	assert.EqualValues(t, "local", Actions.LogStorage.Type)
	assert.EqualValues(t, "/data/gitea/actions_log", Actions.LogStorage.Path)

	assert.EqualValues(t, "local", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "/data/gitea/actions_artifacts", Actions.ArtifactStorage.Path)

	assert.NoError(t, loadAvatarsFrom(cfg))
	assert.EqualValues(t, "local", Avatar.Storage.Type)
	assert.EqualValues(t, "/data/gitea/avatars", Avatar.Storage.Path)

	assert.NoError(t, loadRepoAvatarFrom(cfg))
	assert.EqualValues(t, "local", RepoAvatar.Storage.Type)
	assert.EqualValues(t, "/data/gitea/repo-avatars", RepoAvatar.Storage.Path)
}

func Test_getStorageInheritStorageTypeLocalPathOverrideEmpty(t *testing.T) {
	iniStr := `
[storage]
STORAGE_TYPE = local
PATH = /data/gitea

[repo-archive]
`

	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadPackagesFrom(cfg))
	assert.EqualValues(t, "local", Packages.Storage.Type)
	assert.EqualValues(t, "/data/gitea/packages", Packages.Storage.Path)

	assert.NoError(t, loadRepoArchiveFrom(cfg))
	assert.EqualValues(t, "local", RepoArchive.Storage.Type)
	assert.EqualValues(t, "/data/gitea/repo-archive", RepoArchive.Storage.Path)

	assert.NoError(t, loadActionsFrom(cfg))
	assert.EqualValues(t, "local", Actions.LogStorage.Type)
	assert.EqualValues(t, "/data/gitea/actions_log", Actions.LogStorage.Path)

	assert.EqualValues(t, "local", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "/data/gitea/actions_artifacts", Actions.ArtifactStorage.Path)

	assert.NoError(t, loadAvatarsFrom(cfg))
	assert.EqualValues(t, "local", Avatar.Storage.Type)
	assert.EqualValues(t, "/data/gitea/avatars", Avatar.Storage.Path)

	assert.NoError(t, loadRepoAvatarFrom(cfg))
	assert.EqualValues(t, "local", RepoAvatar.Storage.Type)
	assert.EqualValues(t, "/data/gitea/repo-avatars", RepoAvatar.Storage.Path)
}

func Test_getStorageInheritStorageTypeLocalRelativePathOverride(t *testing.T) {
	iniStr := `
[storage]
STORAGE_TYPE = local
PATH = /data/gitea

[repo-archive]
PATH = archives
`
	AppDataPath = "/tmp/data"
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadPackagesFrom(cfg))
	assert.EqualValues(t, "local", Packages.Storage.Type)
	assert.EqualValues(t, "/data/gitea/packages", Packages.Storage.Path)

	assert.NoError(t, loadRepoArchiveFrom(cfg))
	assert.EqualValues(t, "local", RepoArchive.Storage.Type)
	assert.EqualValues(t, filepath.Join(AppDataPath, "archives"), RepoArchive.Storage.Path)

	assert.NoError(t, loadActionsFrom(cfg))
	assert.EqualValues(t, "local", Actions.LogStorage.Type)
	assert.EqualValues(t, "/data/gitea/actions_log", Actions.LogStorage.Path)

	assert.EqualValues(t, "local", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "/data/gitea/actions_artifacts", Actions.ArtifactStorage.Path)

	assert.NoError(t, loadAvatarsFrom(cfg))
	assert.EqualValues(t, "local", Avatar.Storage.Type)
	assert.EqualValues(t, "/data/gitea/avatars", Avatar.Storage.Path)

	assert.NoError(t, loadRepoAvatarFrom(cfg))
	assert.EqualValues(t, "local", RepoAvatar.Storage.Type)
	assert.EqualValues(t, "/data/gitea/repo-avatars", RepoAvatar.Storage.Path)
}

func Test_getStorageInheritStorageTypeLocalPathOverride2(t *testing.T) {
	iniStr := `
[storage]
STORAGE_TYPE = local
PATH = /data/gitea

[storage.repo-archive]
PATH = /data/gitea/archives
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadPackagesFrom(cfg))
	assert.EqualValues(t, "local", Packages.Storage.Type)
	assert.EqualValues(t, "/data/gitea/packages", Packages.Storage.Path)

	assert.NoError(t, loadRepoArchiveFrom(cfg))
	assert.EqualValues(t, "local", RepoArchive.Storage.Type)
	assert.EqualValues(t, "/data/gitea/archives", RepoArchive.Storage.Path)

	assert.NoError(t, loadActionsFrom(cfg))
	assert.EqualValues(t, "local", Actions.LogStorage.Type)
	assert.EqualValues(t, "/data/gitea/actions_log", Actions.LogStorage.Path)

	assert.EqualValues(t, "local", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "/data/gitea/actions_artifacts", Actions.ArtifactStorage.Path)

	assert.NoError(t, loadAvatarsFrom(cfg))
	assert.EqualValues(t, "local", Avatar.Storage.Type)
	assert.EqualValues(t, "/data/gitea/avatars", Avatar.Storage.Path)

	assert.NoError(t, loadRepoAvatarFrom(cfg))
	assert.EqualValues(t, "local", RepoAvatar.Storage.Type)
	assert.EqualValues(t, "/data/gitea/repo-avatars", RepoAvatar.Storage.Path)
}
