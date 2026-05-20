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
STORAGE_TYPE = my_s3
S3_BUCKET = gitea-attachment

[storage.my_s3]
STORAGE_TYPE = s3
S3_ENDPOINT = my_s3:9000
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))

	assert.EqualValues(t, "s3", Attachment.Storage.Type)
	assert.Equal(t, "my_s3:9000", Attachment.Storage.S3Config.Endpoint)
	assert.Equal(t, "gitea-attachment", Attachment.Storage.S3Config.Bucket)
	assert.Equal(t, "attachments/", Attachment.Storage.S3Config.BasePath)
}

func Test_getStorageTypeSectionOverridesStorageSection(t *testing.T) {
	iniStr := `
[attachment]
STORAGE_TYPE = s3

[storage.s3]
S3_BUCKET = gitea-s3

[storage]
S3_BUCKET = gitea
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))

	assert.EqualValues(t, "s3", Attachment.Storage.Type)
	assert.Equal(t, "gitea-s3", Attachment.Storage.S3Config.Bucket)
	assert.Equal(t, "attachments/", Attachment.Storage.S3Config.BasePath)
}

func Test_getStorageSpecificOverridesStorage(t *testing.T) {
	iniStr := `
[attachment]
STORAGE_TYPE = s3
S3_BUCKET = gitea-attachment

[storage.attachments]
S3_BUCKET = gitea

[storage]
STORAGE_TYPE = local
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))

	assert.EqualValues(t, "s3", Attachment.Storage.Type)
	assert.Equal(t, "gitea-attachment", Attachment.Storage.S3Config.Bucket)
	assert.Equal(t, "attachments/", Attachment.Storage.S3Config.BasePath)
}

func Test_getStorageGetDefaults(t *testing.T) {
	cfg, err := NewConfigProviderFromData("")
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))

	// default storage is local, so bucket is empty
	assert.Empty(t, Attachment.Storage.S3Config.Bucket)
}

func Test_getStorageInheritNameSectionType(t *testing.T) {
	iniStr := `
[storage.attachments]
STORAGE_TYPE = s3
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))

	assert.EqualValues(t, "s3", Attachment.Storage.Type)
}

func Test_AttachmentStorage(t *testing.T) {
	iniStr := `
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
[storage]
STORAGE_TYPE            = s3
S3_ENDPOINT = s3.my-domain.net
S3_BUCKET = gitea
S3_LOCATION = homenet
S3_USE_SSL = true
S3_ACCESS_KEY_ID = correct_key
S3_SECRET_ACCESS_KEY = correct_key
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))
	storage := Attachment.Storage

	assert.EqualValues(t, "s3", storage.Type)
	assert.Equal(t, "gitea", storage.S3Config.Bucket)
}

func Test_AttachmentStorage1(t *testing.T) {
	iniStr := `
[storage]
STORAGE_TYPE = s3
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadAttachmentFrom(cfg))
	assert.EqualValues(t, "s3", Attachment.Storage.Type)
	assert.Equal(t, "gitea", Attachment.Storage.S3Config.Bucket)
	assert.Equal(t, "attachments/", Attachment.Storage.S3Config.BasePath)
}
