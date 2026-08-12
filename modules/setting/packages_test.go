// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"slices"
	"testing"

	"gitea.dev/modules/test"

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
	defer test.MockVariableValue(&Packages)()

	testConfigLoad(t, []any{loadPackagesFrom}, []configTestCase{
		{
			name: "inherits from [storage] if nothing is configured",
			ini:  "[storage]\nSTORAGE_TYPE = minio",
			want: minioStorageAt("packages", &Packages.Storage, "packages/"),
		},
		{
			name: "[storage.packages] configures it directly",
			ini:  "[storage.packages]\nSTORAGE_TYPE = minio",
			want: minioStorageAt("packages", &Packages.Storage, "packages/"),
		},
		{
			name: "[packages].STORAGE_TYPE can name another storage",
			ini:  "[packages]\nSTORAGE_TYPE = my_minio\n\n[storage.my_minio]\nSTORAGE_TYPE = minio",
			want: minioStorageAt("packages", &Packages.Storage, "packages/"),
		},
		{
			name: "[packages].MINIO_BASE_PATH overrides the named storage",
			ini:  "[packages]\nSTORAGE_TYPE = my_minio\nMINIO_BASE_PATH = my_packages/\n\n[storage.my_minio]\nSTORAGE_TYPE = minio",
			want: minioStorageAt("packages", &Packages.Storage, "my_packages/"),
		},
	})
}

func Test_PackageStorage(t *testing.T) {
	defer test.MockVariableValue(&Packages)()

	minioSection := `
STORAGE_TYPE            = minio
MINIO_ENDPOINT          = s3.my-domain.net
MINIO_BUCKET            = gitea
MINIO_LOCATION          = homenet
MINIO_USE_SSL           = true
MINIO_ACCESS_KEY_ID     = correct_key
MINIO_SECRET_ACCESS_KEY = correct_key
`
	served := func(basePath string) []configCheck {
		return slices.Concat(
			minioStorage("packages", &Packages.Storage, "gitea", basePath),
			[]configCheck{fieldOf("SERVE_DIRECT", func() bool { return Packages.Storage.MinioConfig.ServeDirect }, true)},
		)
	}

	testConfigLoad(t, []any{loadPackagesFrom}, []configTestCase{
		{
			name: "[packages] over a global [storage]",
			ini:  ";;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;\n[packages]\nMINIO_BASE_PATH = packages/\nSERVE_DIRECT = true\n[storage]" + minioSection,
			want: served("packages/"),
		},
		{
			name: "[storage.packages] over a global [storage]",
			ini:  ";;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;\n[storage.packages]\nMINIO_BASE_PATH = packages/\nSERVE_DIRECT = true\n[storage]" + minioSection,
			want: served("packages/"),
		},
		{
			name: "[packages] pointing at a named storage",
			ini:  ";;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;\n[packages]\nSTORAGE_TYPE            = my_cfg\nMINIO_BASE_PATH = my_packages/\nSERVE_DIRECT = true\n[storage.my_cfg]" + minioSection,
			want: served("my_packages/"),
		},
		{
			name: "[storage.packages] pointing at a named storage",
			ini:  ";;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;\n[storage.packages]\nSTORAGE_TYPE            = my_cfg\nMINIO_BASE_PATH = my_packages/\nSERVE_DIRECT = true\n[storage.my_cfg]" + minioSection,
			want: served("my_packages/"),
		},
	})
}
