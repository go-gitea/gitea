// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"bytes"
	"io"
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

func Test_azureBlobObject(t *testing.T) {
	if os.Getenv("CI") == "" {
		t.Skip("azureBlobStorage not present outside of CI")
		return
	}

	s, err := NewStorage(setting.AzureBlobStorageType, &setting.Storage{
		AzureBlobConfig: setting.AzureBlobStorageConfig{
			// https://learn.microsoft.com/azure/storage/common/storage-use-azurite?tabs=visual-studio-code#ip-style-url
			Endpoint: "http://devstoreaccount1.azurite.local:10000",
			// https://learn.microsoft.com/azure/storage/common/storage-use-azurite?tabs=visual-studio-code#well-known-storage-account-and-key
			AccountName: "devstoreaccount1",
			AccountKey:  "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==",
			Container:   "test",
		},
	})
	assert.NoError(t, err)

	data := "Q2xTckt6Y1hDOWh0"
	_, err = s.Save("test.txt", bytes.NewBufferString(data), int64(len(data)))
	assert.NoError(t, err)
	obj, err := s.Open("test.txt")
	assert.NoError(t, err)
	offset, err := obj.Seek(2, io.SeekStart)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, offset)
	buf1 := make([]byte, 3)
	read, err := obj.Read(buf1)
	assert.NoError(t, err)
	assert.EqualValues(t, 3, read)
	assert.Equal(t, data[2:5], string(buf1))
	offset, err = obj.Seek(-5, io.SeekEnd)
	assert.NoError(t, err)
	assert.EqualValues(t, len(data)-5, offset)
	buf2 := make([]byte, 4)
	read, err = obj.Read(buf2)
	assert.NoError(t, err)
	assert.EqualValues(t, 4, read)
	assert.Equal(t, data[11:15], string(buf2))
	assert.NoError(t, obj.Close())
	assert.NoError(t, s.Delete("test.txt"))
}
