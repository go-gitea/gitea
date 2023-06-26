// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
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
