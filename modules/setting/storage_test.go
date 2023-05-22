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
MINIO_BUCKET = gitea-storage
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))
	assert.EqualValues(t, "gitea-attachment", Attachment.Storage.Section.Key("MINIO_BUCKET").String())

	assert.NoError(t, loadLFSFrom(cfg))
	assert.EqualValues(t, "gitea-lfs", LFS.Storage.Section.Key("MINIO_BUCKET").String())

	assert.NoError(t, loadAvatarsFrom(cfg))
	assert.EqualValues(t, "gitea-storage", Avatar.Storage.Section.Key("MINIO_BUCKET").String())
}

func Test_getStorageUseOtherNameAsType(t *testing.T) {
	iniStr := `
[attachment]
STORAGE_TYPE = lfs

[storage.lfs]
MINIO_BUCKET = gitea-storage
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))
	assert.EqualValues(t, "gitea-storage", Attachment.Storage.Section.Key("MINIO_BUCKET").String())

	assert.NoError(t, loadLFSFrom(cfg))
	assert.EqualValues(t, "gitea-storage", LFS.Storage.Section.Key("MINIO_BUCKET").String())
}

func Test_getStorageInheritStorageType(t *testing.T) {
	iniStr := `
[storage]
STORAGE_TYPE = minio
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))
	assert.EqualValues(t, "minio", Attachment.Storage.Type)
	assert.EqualValues(t, "gitea", Attachment.Storage.Section.Key("MINIO_BUCKET").String())
	assert.EqualValues(t, "attachments/", Attachment.Storage.Section.Key("MINIO_BASE_PATH").MustString(""))

	assert.NoError(t, loadLFSFrom(cfg))
	assert.EqualValues(t, "minio", LFS.Storage.Type)
	assert.EqualValues(t, "gitea", LFS.Storage.Section.Key("MINIO_BUCKET").String())
	assert.EqualValues(t, "lfs/", LFS.Storage.Section.Key("MINIO_BASE_PATH").MustString(""))

	assert.NoError(t, loadPackagesFrom(cfg))
	assert.EqualValues(t, "minio", Packages.Storage.Type)
	assert.EqualValues(t, "gitea", Packages.Storage.Section.Key("MINIO_BUCKET").String())
	assert.EqualValues(t, "packages/", Packages.Storage.Section.Key("MINIO_BASE_PATH").MustString(""))

	assert.NoError(t, loadRepoArchiveFrom(cfg))
	assert.EqualValues(t, "minio", RepoArchive.Storage.Type)
	assert.EqualValues(t, "gitea", RepoArchive.Storage.Section.Key("MINIO_BUCKET").String())
	assert.EqualValues(t, "repo-archive/", RepoArchive.Storage.Section.Key("MINIO_BASE_PATH").MustString(""))

	assert.NoError(t, loadActionsFrom(cfg))
	assert.EqualValues(t, "minio", Actions.LogStorage.Type)
	assert.EqualValues(t, "gitea", Actions.LogStorage.Section.Key("MINIO_BUCKET").String())
	assert.EqualValues(t, "actions_log/", Actions.LogStorage.Section.Key("MINIO_BASE_PATH").MustString(""))

	assert.NoError(t, loadAvatarsFrom(cfg))
	assert.EqualValues(t, "minio", Avatar.Storage.Type)
	assert.EqualValues(t, "gitea", Avatar.Storage.Section.Key("MINIO_BUCKET").String())
	assert.EqualValues(t, "avatars/", Avatar.Storage.Section.Key("MINIO_BASE_PATH").MustString(""))

	assert.NoError(t, loadRepoAvatarFrom(cfg))
	assert.EqualValues(t, "minio", RepoAvatar.Storage.Type)
	assert.EqualValues(t, "gitea", RepoAvatar.Storage.Section.Key("MINIO_BUCKET").String())
	assert.EqualValues(t, "repo-avatars/", RepoAvatar.Storage.Section.Key("MINIO_BASE_PATH").MustString(""))

	iniStr = `
[storage.attachments]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	sec := cfg.Section("attachment")
	storageType := sec.Key("STORAGE_TYPE").MustString("")
	storage, err := getStorage(cfg, "attachments", sec, storageType)
	assert.NoError(t, err)

	assert.EqualValues(t, "minio", storage.Type)
}
