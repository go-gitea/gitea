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

func Test_PackageStorage(t *testing.T) {
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

	assert.NoError(t, loadPackagesFrom(cfg))
	storage := Packages.Storage

	assert.EqualValues(t, "minio", storage.Type)
	assert.EqualValues(t, "gitea", storage.Section.Key("MINIO_BUCKET").String())
}
