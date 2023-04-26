// Copyright 2020 The Gitea Authors. All rights reserved.
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
	cfg, err := newConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	sec := cfg.Section("attachment")
	storageType := sec.Key("STORAGE_TYPE").MustString("")
	storage := getStorage(cfg, "attachments", storageType, sec)

	assert.EqualValues(t, "minio", storage.Type)
	assert.EqualValues(t, "my_minio:9000", storage.Section.Key("MINIO_ENDPOINT").String())
	assert.EqualValues(t, "gitea-attachment", storage.Section.Key("MINIO_BUCKET").String())
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
	cfg, err := newConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	sec := cfg.Section("attachment")
	storageType := sec.Key("STORAGE_TYPE").MustString("")
	storage := getStorage(cfg, "attachments", storageType, sec)

	assert.EqualValues(t, "minio", storage.Type)
	assert.EqualValues(t, "gitea-attachment", storage.Section.Key("MINIO_BUCKET").String())
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
	cfg, err := newConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	sec := cfg.Section("attachment")
	storageType := sec.Key("STORAGE_TYPE").MustString("")
	storage := getStorage(cfg, "attachments", storageType, sec)

	assert.EqualValues(t, "minio", storage.Type)
	assert.EqualValues(t, "gitea-minio", storage.Section.Key("MINIO_BUCKET").String())
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
	cfg, err := newConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	sec := cfg.Section("attachment")
	storageType := sec.Key("STORAGE_TYPE").MustString("")
	storage := getStorage(cfg, "attachments", storageType, sec)

	assert.EqualValues(t, "minio", storage.Type)
	assert.EqualValues(t, "gitea-attachment", storage.Section.Key("MINIO_BUCKET").String())
}

func Test_getStorageGetDefaults(t *testing.T) {
	cfg, err := newConfigProviderFromData("")
	assert.NoError(t, err)

	sec := cfg.Section("attachment")
	storageType := sec.Key("STORAGE_TYPE").MustString("")
	storage := getStorage(cfg, "attachments", storageType, sec)

	assert.EqualValues(t, "gitea", storage.Section.Key("MINIO_BUCKET").String())
}

func Test_getStorageMultipleName(t *testing.T) {
	iniStr := `
[lfs]
MINIO_BUCKET = gitea-lfs

[attachment]
MINIO_BUCKET = gitea-attachment

[storage]
MINIO_BUCKET = gitea-storage
`
	cfg, err := newConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	{
		sec := cfg.Section("attachment")
		storageType := sec.Key("STORAGE_TYPE").MustString("")
		storage := getStorage(cfg, "attachments", storageType, sec)

		assert.EqualValues(t, "gitea-attachment", storage.Section.Key("MINIO_BUCKET").String())
	}
	{
		sec := cfg.Section("lfs")
		storageType := sec.Key("STORAGE_TYPE").MustString("")
		storage := getStorage(cfg, "lfs", storageType, sec)

		assert.EqualValues(t, "gitea-lfs", storage.Section.Key("MINIO_BUCKET").String())
	}
	{
		sec := cfg.Section("avatar")
		storageType := sec.Key("STORAGE_TYPE").MustString("")
		storage := getStorage(cfg, "avatars", storageType, sec)

		assert.EqualValues(t, "gitea-storage", storage.Section.Key("MINIO_BUCKET").String())
	}
}

func Test_getStorageUseOtherNameAsType(t *testing.T) {
	iniStr := `
[attachment]
STORAGE_TYPE = lfs

[storage.lfs]
MINIO_BUCKET = gitea-storage
`
	cfg, err := newConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	{
		sec := cfg.Section("attachment")
		storageType := sec.Key("STORAGE_TYPE").MustString("")
		storage := getStorage(cfg, "attachments", storageType, sec)

		assert.EqualValues(t, "gitea-storage", storage.Section.Key("MINIO_BUCKET").String())
	}
	{
		sec := cfg.Section("lfs")
		storageType := sec.Key("STORAGE_TYPE").MustString("")
		storage := getStorage(cfg, "lfs", storageType, sec)

		assert.EqualValues(t, "gitea-storage", storage.Section.Key("MINIO_BUCKET").String())
	}
}

func Test_getStorageInheritStorageType(t *testing.T) {
	iniStr := `
[storage]
STORAGE_TYPE = minio
`
	cfg, err := newConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	sec := cfg.Section("attachment")
	storageType := sec.Key("STORAGE_TYPE").MustString("")
	storage := getStorage(cfg, "attachments", storageType, sec)

	assert.EqualValues(t, "minio", storage.Type)
}

func Test_getStorageInheritNameSectionType(t *testing.T) {
	iniStr := `
[storage.attachments]
STORAGE_TYPE = minio
`
	cfg, err := newConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	sec := cfg.Section("attachment")
	storageType := sec.Key("STORAGE_TYPE").MustString("")
	storage := getStorage(cfg, "attachments", storageType, sec)

	assert.EqualValues(t, "minio", storage.Type)
}
