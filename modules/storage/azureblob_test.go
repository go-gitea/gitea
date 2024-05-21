// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"os"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestAzureBlobStorageIterator(t *testing.T) {
	if os.Getenv("CI") == "" {
		t.Skip("azureBlobStorage not present outside of CI")
		return
	}
	testStorageIterator(t, setting.AzureBlobStorageType, &setting.Storage{
		AzureBlobConfig: setting.AzureBlobStorageConfig{
			// https://learn.microsoft.com/azure/storage/common/storage-use-azurite?tabs=visual-studio-code#ip-style-url
			Endpoint: "http://devstoreaccount1.azurite.local:10000",
			// https://learn.microsoft.com/azure/storage/common/storage-use-azurite?tabs=visual-studio-code#well-known-storage-account-and-key
			AccountName: "devstoreaccount1",
			AccountKey:  "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==",
			Container:   "test",
		},
	})
}

func TestAzureBlobStoragePath(t *testing.T) {
	m := &AzureBlobStorage{cfg: &setting.AzureBlobStorageConfig{BasePath: ""}}
	assert.Equal(t, "", m.buildAzureBlobPath("/"))
	assert.Equal(t, "", m.buildAzureBlobPath("."))
	assert.Equal(t, "a", m.buildAzureBlobPath("/a"))
	assert.Equal(t, "a/b", m.buildAzureBlobPath("/a/b/"))

	m = &AzureBlobStorage{cfg: &setting.AzureBlobStorageConfig{BasePath: "/"}}
	assert.Equal(t, "", m.buildAzureBlobPath("/"))
	assert.Equal(t, "", m.buildAzureBlobPath("."))
	assert.Equal(t, "a", m.buildAzureBlobPath("/a"))
	assert.Equal(t, "a/b", m.buildAzureBlobPath("/a/b/"))

	m = &AzureBlobStorage{cfg: &setting.AzureBlobStorageConfig{BasePath: "/base"}}
	assert.Equal(t, "base", m.buildAzureBlobPath("/"))
	assert.Equal(t, "base", m.buildAzureBlobPath("."))
	assert.Equal(t, "base/a", m.buildAzureBlobPath("/a"))
	assert.Equal(t, "base/a/b", m.buildAzureBlobPath("/a/b/"))

	m = &AzureBlobStorage{cfg: &setting.AzureBlobStorageConfig{BasePath: "/base/"}}
	assert.Equal(t, "base", m.buildAzureBlobPath("/"))
	assert.Equal(t, "base", m.buildAzureBlobPath("."))
	assert.Equal(t, "base/a", m.buildAzureBlobPath("/a"))
	assert.Equal(t, "base/a/b", m.buildAzureBlobPath("/a/b/"))
}
