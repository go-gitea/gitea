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
	assert.EqualValues(t, "my_minio:9000", Attachment.Storage.MinioConfig.Endpoint)
	assert.EqualValues(t, "gitea-attachment", Attachment.Storage.MinioConfig.Bucket)
	assert.EqualValues(t, "attachments/", Attachment.Storage.MinioConfig.BasePath)
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
	assert.EqualValues(t, "gitea-minio", Attachment.Storage.MinioConfig.Bucket)
	assert.EqualValues(t, "attachments/", Attachment.Storage.MinioConfig.BasePath)
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
	assert.EqualValues(t, "gitea-attachment", Attachment.Storage.MinioConfig.Bucket)
	assert.EqualValues(t, "attachments/", Attachment.Storage.MinioConfig.BasePath)
}

func Test_getStorageGetDefaults(t *testing.T) {
	cfg, err := NewConfigProviderFromData("")
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))

	// default storage is local, so bucket is empty
	assert.EqualValues(t, "", Attachment.Storage.MinioConfig.Bucket)
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

func Test_AttachmentStorage(t *testing.T) {
	iniStr := `
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
[storage]
STORAGE_TYPE            = minio
MINIO_ENDPOINT          = s3.my-domain.net
MINIO_BUCKET            = gitea
MINIO_LOCATION          = homenet
MINIO_USE_SSL           = true
MINIO_ACCESS_KEY_ID     = correct_key
MINIO_SECRET_ACCESS_KEY = correct_key
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))
	storage := Attachment.Storage

	assert.EqualValues(t, "minio", storage.Type)
	assert.EqualValues(t, "gitea", storage.MinioConfig.Bucket)
}

func Test_AttachmentStorage1(t *testing.T) {
	iniStr := `
[storage]
STORAGE_TYPE = minio
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))
	assert.EqualValues(t, "minio", Attachment.Storage.Type)
	assert.EqualValues(t, "gitea", Attachment.Storage.MinioConfig.Bucket)
	assert.EqualValues(t, "attachments/", Attachment.Storage.MinioConfig.BasePath)
}
