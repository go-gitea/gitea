// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMustBytes(t *testing.T) {
	test := func(value string) int64 {
		cfg, err := NewConfigProviderFromData("[test]")
		assert.NoError(t, err)
		sec := cfg.Section("test")
		sec.NewKey("VALUE", value)

		return mustBytes(sec, "VALUE")
	}

	assert.EqualValues(t, -1, test(""))
	assert.EqualValues(t, -1, test("-1"))
	assert.EqualValues(t, 0, test("0"))
	assert.EqualValues(t, 1, test("1"))
	assert.EqualValues(t, 10000, test("10000"))
	assert.EqualValues(t, 1000000, test("1 mb"))
	assert.EqualValues(t, 1048576, test("1mib"))
	assert.EqualValues(t, 1782579, test("1.7mib"))
	assert.EqualValues(t, -1, test("1 yib")) // too large
}

func Test_getStorageInheritNameSectionTypeForPackages(t *testing.T) {
	// packages storage inherits from storage if nothing configured
	iniStr := `
[storage]
STORAGE_TYPE = minio
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadPackagesFrom(cfg))

	assert.EqualValues(t, "minio", Packages.Storage.Type)
	assert.EqualValues(t, "packages/", cfg.Section("packages").Key("MINIO_BASE_PATH").MustString(""))

	// we can also configure packages storage directly
	iniStr = `
[storage.packages]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadPackagesFrom(cfg))

	assert.EqualValues(t, "minio", Packages.Storage.Type)
	assert.EqualValues(t, "packages/", cfg.Section("packages").Key("MINIO_BASE_PATH").MustString(""))

	// or we can indicate the storage type in the packages section
	iniStr = `
[packages]
STORAGE_TYPE = my_minio

[storage.my_minio]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadPackagesFrom(cfg))

	assert.EqualValues(t, "minio", Packages.Storage.Type)
	assert.EqualValues(t, "packages/", cfg.Section("packages").Key("MINIO_BASE_PATH").MustString(""))

	// or we can indicate the storage type  and minio base path in the packages section
	iniStr = `
[packages]
STORAGE_TYPE = my_minio
MINIO_BASE_PATH = my_packages/

[storage.my_minio]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadPackagesFrom(cfg))

	assert.EqualValues(t, "minio", Packages.Storage.Type)
	assert.EqualValues(t, "my_packages/", cfg.Section("packages").Key("MINIO_BASE_PATH").MustString(""))
}
