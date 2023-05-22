// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getStorageCustomType(t *testing.T) {
	iniStr := `
[attachment]
STORAGE_TYPE = my_minio
MINIO_BUCKET = gitea-attachment

[storage.my_minio]
STORAGE_TYPE = minio
MINIO_ENDPOINT = my_minio:9000
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))

	assert.EqualValues(t, "minio", Attachment.Storage.Type)
	assert.EqualValues(t, "my_minio:9000", Attachment.Storage.Section.Key("MINIO_ENDPOINT").String())
	assert.EqualValues(t, "gitea-attachment", Attachment.Storage.Section.Key("MINIO_BUCKET").String())
}

func Test_getStorageNameSectionOverridesTypeSection(t *testing.T) {
	iniStr := `
[attachment]
STORAGE_TYPE = minio

[storage.attachments]
MINIO_BUCKET = gitea-attachment

[storage.minio]
MINIO_BUCKET = gitea
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))

	assert.EqualValues(t, "minio", Attachment.Storage.Type)
	assert.EqualValues(t, "gitea-attachment", Attachment.Storage.Section.Key("MINIO_BUCKET").String())
}

func Test_getStorageTypeSectionOverridesStorageSection(t *testing.T) {
	iniStr := `
[attachment]
STORAGE_TYPE = minio

[storage.minio]
MINIO_BUCKET = gitea-minio

[storage]
MINIO_BUCKET = gitea
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))

	assert.EqualValues(t, "minio", Attachment.Storage.Type)
	assert.EqualValues(t, "gitea-minio", Attachment.Storage.Section.Key("MINIO_BUCKET").String())
}

func Test_getStorageSpecificOverridesStorage(t *testing.T) {
	iniStr := `
[attachment]
STORAGE_TYPE = minio
MINIO_BUCKET = gitea-attachment

[storage.attachments]
MINIO_BUCKET = gitea

[storage]
STORAGE_TYPE = local
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))

	assert.EqualValues(t, "minio", Attachment.Storage.Type)
	assert.EqualValues(t, "gitea-attachment", Attachment.Storage.Section.Key("MINIO_BUCKET").String())
}

func Test_getStorageGetDefaults(t *testing.T) {
	cfg, err := NewConfigProviderFromData("")
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))

	assert.EqualValues(t, "gitea", Attachment.Storage.Section.Key("MINIO_BUCKET").String())
}

func Test_getStorageInheritNameSectionType(t *testing.T) {
	iniStr := `
[storage.attachments]
STORAGE_TYPE = minio
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))

	assert.EqualValues(t, "minio", Attachment.Storage.Type)
}
