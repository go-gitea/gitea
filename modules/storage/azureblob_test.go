// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"io"
	"strings"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prepareAzureStorageConfig(t *testing.T, basePath ...string) *setting.Storage {
	endpoint := test.ExternalServiceHTTP(t, "TEST_AZURESTORAGE_ENDPOINT", "http://devstoreaccount1.azurite.local:10000")
	return &setting.Storage{
		AzureBlobConfig: setting.AzureBlobStorageConfig{
			// https://learn.microsoft.com/azure/storage/common/storage-use-azurite?tabs=visual-studio-code#ip-style-url
			Endpoint: endpoint,
			// https://learn.microsoft.com/azure/storage/common/storage-use-azurite?tabs=visual-studio-code#well-known-storage-account-and-key
			AccountName: "devstoreaccount1",
			AccountKey:  "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==",
			Container:   "test-container",
			BasePath:    util.OptionalArg(basePath),
		},
	}
}

func TestAzureBlobStorage(t *testing.T) {
	config := prepareAzureStorageConfig(t)
	table := []struct {
		name string
		test func(t *testing.T, typStr Type, cfg *setting.Storage)
	}{
		{
			name: "iterator",
			test: testStorageIterator,
		},
		{
			name: "testBlobStorageURLContentTypeAndDisposition",
			test: testBlobStorageURLContentTypeAndDisposition,
		},
	}
	for _, entry := range table {
		t.Run(entry.name, func(t *testing.T) {
			entry.test(t, setting.AzureBlobStorageType, config)
		})
	}
}

func TestAzureBlobStoragePath(t *testing.T) {
	m := &AzureBlobStorage{cfg: &setting.AzureBlobStorageConfig{BasePath: ""}}
	assert.Empty(t, m.buildAzureBlobPath("/"))
	assert.Empty(t, m.buildAzureBlobPath("."))
	assert.Equal(t, "a", m.buildAzureBlobPath("/a"))
	assert.Equal(t, "a/b", m.buildAzureBlobPath("/a/b/"))

	m = &AzureBlobStorage{cfg: &setting.AzureBlobStorageConfig{BasePath: "/"}}
	assert.Empty(t, m.buildAzureBlobPath("/"))
	assert.Empty(t, m.buildAzureBlobPath("."))
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
	s, err := NewStorage(setting.AzureBlobStorageType, prepareAzureStorageConfig(t))
	require.NoError(t, err)

	data := "Q2xTckt6Y1hDOWh0"
	_, err = s.Save("test.txt", strings.NewReader(data), int64(len(data)))
	assert.NoError(t, err)
	obj, err := s.Open("test.txt")
	assert.NoError(t, err)
	offset, err := obj.Seek(2, io.SeekStart)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, offset)
	buf1 := make([]byte, 3)
	read, err := obj.Read(buf1)
	assert.NoError(t, err)
	assert.Equal(t, 3, read)
	assert.Equal(t, data[2:5], string(buf1))
	offset, err = obj.Seek(-5, io.SeekEnd)
	assert.NoError(t, err)
	assert.EqualValues(t, len(data)-5, offset)
	buf2 := make([]byte, 4)
	read, err = obj.Read(buf2)
	assert.NoError(t, err)
	assert.Equal(t, 4, read)
	assert.Equal(t, data[11:15], string(buf2))
	assert.NoError(t, obj.Close())
	assert.NoError(t, s.Delete("test.txt"))
}

func TestAzureBlobStorageDumpArchive(t *testing.T) {
	s, err := NewStorage(setting.AzureBlobStorageType, prepareAzureStorageConfig(t, "test-base-path"))
	require.NoError(t, err)

	const objPath = "aa/bb/deadbeefcafe"
	const content = "regression test content for issue 35476"
	_, err = s.Save(objPath, strings.NewReader(content), int64(len(content)))
	require.NoError(t, err)
	defer s.Delete(objPath)

	err = s.IterateObjects("", func(path string, object Object) error {
		assert.Equal(t, "/"+objPath, path)

		data, err := io.ReadAll(object)
		require.NoError(t, err)

		assert.Equal(t, content, string(data))
		return nil
	})
	require.NoError(t, err)
}
