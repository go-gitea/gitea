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
	for _, testcase := range []struct {
		iniStr, basePath string
	}{
		// packages storage inherits from storage if nothing configured
		{`
[storage]
STORAGE_TYPE = minio
`, "packages/"},
		// we can also configure packages storage directly
		{`
[storage.packages]
STORAGE_TYPE = minio
`, "packages/"},
		// or we can indicate the storage type in the packages section
		{`
[packages]
STORAGE_TYPE = my_minio

[storage.my_minio]
STORAGE_TYPE = minio
`, "packages/"},
		// or we can indicate the storage type  and minio base path in the packages section
		{`
[packages]
STORAGE_TYPE = my_minio
MINIO_BASE_PATH = my_packages/

[storage.my_minio]
STORAGE_TYPE = minio
`, "my_packages/"},
	} {
		cfg, err := NewConfigProviderFromData(testcase.iniStr)
		assert.NoError(t, err)
		assert.NoError(t, loadPackagesFrom(cfg))

		assert.EqualValues(t, "minio", Packages.Storage.Type)
		assert.Equal(t, testcase.basePath, Packages.Storage.MinioConfig.BasePath)
	}
}

func Test_PackageStorage(t *testing.T) {
	const storageConfig = `STORAGE_TYPE            = minio
MINIO_ENDPOINT          = s3.my-domain.net
MINIO_BUCKET            = gitea
MINIO_LOCATION          = homenet
MINIO_USE_SSL           = true
MINIO_ACCESS_KEY_ID     = correct_key
MINIO_SECRET_ACCESS_KEY = correct_key
`
	for _, testcase := range []struct {
		iniStr, basePath string
	}{
		{`
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
[packages]
MINIO_BASE_PATH = packages/
SERVE_DIRECT = true
[storage]
`, "packages/"},
		{`
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
[storage.packages]
MINIO_BASE_PATH = packages/
SERVE_DIRECT = true
[storage]
`, "packages/"},
		{`
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
[packages]
STORAGE_TYPE            = my_cfg
MINIO_BASE_PATH = my_packages/
SERVE_DIRECT = true
[storage.my_cfg]
`, "my_packages/"},
		{`
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
[storage.packages]
STORAGE_TYPE            = my_cfg
MINIO_BASE_PATH = my_packages/
SERVE_DIRECT = true
[storage.my_cfg]
`, "my_packages/"},
	} {
		cfg, err := NewConfigProviderFromData(testcase.iniStr + storageConfig)
		assert.NoError(t, err)

		assert.NoError(t, loadPackagesFrom(cfg))
		storage := Packages.Storage

		assert.EqualValues(t, "minio", storage.Type)
		assert.Equal(t, "gitea", storage.MinioConfig.Bucket)
		assert.Equal(t, testcase.basePath, storage.MinioConfig.BasePath)
		assert.True(t, storage.MinioConfig.ServeDirect)
	}
}
