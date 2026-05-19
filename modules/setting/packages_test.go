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
STORAGE_TYPE = s3
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadPackagesFrom(cfg))

	assert.EqualValues(t, "s3", Packages.Storage.Type)
	assert.Equal(t, "packages/", Packages.Storage.S3Config.BasePath)

	// we can also configure packages storage directly
	iniStr = `
[storage.packages]
STORAGE_TYPE = s3
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadPackagesFrom(cfg))

	assert.EqualValues(t, "s3", Packages.Storage.Type)
	assert.Equal(t, "packages/", Packages.Storage.S3Config.BasePath)

	// or we can indicate the storage type in the packages section
	iniStr = `
[packages]
STORAGE_TYPE = my_s3

[storage.my_s3]
STORAGE_TYPE = s3
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadPackagesFrom(cfg))

	assert.EqualValues(t, "s3", Packages.Storage.Type)
	assert.Equal(t, "packages/", Packages.Storage.S3Config.BasePath)

	// or we can indicate the storage type and S3 base path in the packages section
	iniStr = `
[packages]
STORAGE_TYPE = my_s3
S3_BASE_PATH = my_packages/

[storage.my_s3]
STORAGE_TYPE = s3
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadPackagesFrom(cfg))

	assert.EqualValues(t, "s3", Packages.Storage.Type)
	assert.Equal(t, "my_packages/", Packages.Storage.S3Config.BasePath)
}

func Test_PackageStorage1(t *testing.T) {
	iniStr := `
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
[packages]
S3_BASE_PATH = packages/
SERVE_DIRECT = true
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

	assert.NoError(t, loadPackagesFrom(cfg))
	storage := Packages.Storage

	assert.EqualValues(t, "s3", storage.Type)
	assert.Equal(t, "gitea", storage.S3Config.Bucket)
	assert.Equal(t, "packages/", storage.S3Config.BasePath)
	assert.True(t, storage.S3Config.ServeDirect)
}

func Test_PackageStorage2(t *testing.T) {
	iniStr := `
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
[storage.packages]
S3_BASE_PATH = packages/
SERVE_DIRECT = true
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

	assert.NoError(t, loadPackagesFrom(cfg))
	storage := Packages.Storage

	assert.EqualValues(t, "s3", storage.Type)
	assert.Equal(t, "gitea", storage.S3Config.Bucket)
	assert.Equal(t, "packages/", storage.S3Config.BasePath)
	assert.True(t, storage.S3Config.ServeDirect)
}

func Test_PackageStorage3(t *testing.T) {
	iniStr := `
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
[packages]
STORAGE_TYPE            = my_cfg
S3_BASE_PATH = my_packages/
SERVE_DIRECT = true
[storage.my_cfg]
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

	assert.NoError(t, loadPackagesFrom(cfg))
	storage := Packages.Storage

	assert.EqualValues(t, "s3", storage.Type)
	assert.Equal(t, "gitea", storage.S3Config.Bucket)
	assert.Equal(t, "my_packages/", storage.S3Config.BasePath)
	assert.True(t, storage.S3Config.ServeDirect)
}

func Test_PackageStorage4(t *testing.T) {
	iniStr := `
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
[storage.packages]
STORAGE_TYPE            = my_cfg
S3_BASE_PATH = my_packages/
SERVE_DIRECT = true
[storage.my_cfg]
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

	assert.NoError(t, loadPackagesFrom(cfg))
	storage := Packages.Storage

	assert.EqualValues(t, "s3", storage.Type)
	assert.Equal(t, "gitea", storage.S3Config.Bucket)
	assert.Equal(t, "my_packages/", storage.S3Config.BasePath)
	assert.True(t, storage.S3Config.ServeDirect)
}
