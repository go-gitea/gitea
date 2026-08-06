// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"path"
	"strings"
	"testing"

	"gitea.dev/modules/dump"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestAzureBlobStorage(t *testing.T) {
	endpoint := test.ExternalServiceHTTP(t, "TEST_AZURESTORAGE_ENDPOINT", "http://devstoreaccount1.azurite.local:10000")
	storageType := setting.AzureBlobStorageType
	config := &setting.Storage{
		AzureBlobConfig: setting.AzureBlobStorageConfig{
			// https://learn.microsoft.com/azure/storage/common/storage-use-azurite?tabs=visual-studio-code#ip-style-url
			Endpoint: endpoint,
			// https://learn.microsoft.com/azure/storage/common/storage-use-azurite?tabs=visual-studio-code#well-known-storage-account-and-key
			AccountName: "devstoreaccount1",
			AccountKey:  "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==",
			Container:   "test",
		},
	}
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
			entry.test(t, storageType, config)
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
	endpoint := test.ExternalServiceHTTP(t, "TEST_AZURESTORAGE_ENDPOINT", "http://devstoreaccount1.azurite.local:10000")
	s, err := NewStorage(setting.AzureBlobStorageType, &setting.Storage{
		AzureBlobConfig: setting.AzureBlobStorageConfig{
			// https://learn.microsoft.com/azure/storage/common/storage-use-azurite?tabs=visual-studio-code#ip-style-url
			Endpoint: endpoint,
			// https://learn.microsoft.com/azure/storage/common/storage-use-azurite?tabs=visual-studio-code#well-known-storage-account-and-key
			AccountName: "devstoreaccount1",
			AccountKey:  "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==",
			Container:   "test",
		},
	})
	assert.NoError(t, err)

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

// TestAzureBlobStorageDumpArchive is a regression test for
// https://github.com/go-gitea/gitea/issues/35476 ("Unable to dump files
// from Azure Blob Storage").
//
// It exercises the complete dump/archive path used by "gitea dump" for LFS
// objects:
//
//	storage.IterateObjects() -> Stat() -> dump.Dumper.AddFileByReader() -> archives
//
// and asserts that the produced archive actually contains the object with
// its correct content, rather than only checking the paths returned by
// IterateObjects (as testStorageIterator/Test_azureBlobObject already do).
//
// Crucially, this test configures a non-empty BasePath, which is what the
// default Gitea configuration uses for LFS storage. With BasePath == "" (as
// used by the other tests in this file) the underlying bug is invisible,
// because prepending an empty base path twice is a no-op.
func TestAzureBlobStorageDumpArchive(t *testing.T) {
	endpoint := test.ExternalServiceHTTP(t, "TEST_AZURESTORAGE_ENDPOINT", "http://devstoreaccount1.azurite.local:10000")
	s, err := NewStorage(setting.AzureBlobStorageType, &setting.Storage{
		AzureBlobConfig: setting.AzureBlobStorageConfig{
			Endpoint:    endpoint,
			AccountName: "devstoreaccount1",
			AccountKey:  "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==",
			Container:   "test",
			// A non-empty base path is required to reproduce the double-prefix bug.
			BasePath: "gitea-lfs-dump-test",
		},
	})
	assert.NoError(t, err)

	const objPath = "aa/bb/deadbeefcafe"
	const content = "regression test content for issue 35476"
	_, err = s.Save(objPath, strings.NewReader(content), int64(len(content)))
	assert.NoError(t, err)
	defer s.Delete(objPath) //nolint:errcheck

	var archiveBuf bytes.Buffer
	dumper, err := dump.NewDumper(context.Background(), "zip", &archiveBuf)
	assert.NoError(t, err)

	nameInArchive := path.Join("data", "lfs", objPath)
	err = s.IterateObjects("", func(objPath string, object Object) error {
		info, err := object.Stat()
		if err != nil {
			return err
		}
		return dumper.AddFileByReader(object, info, path.Join("data", "lfs", objPath))
	})
	assert.NoError(t, err)
	assert.NoError(t, dumper.Close())
	if err != nil {
		// Don't try to parse a truncated/invalid archive if the dump above already failed.
		return
	}

	zr, err := zip.NewReader(bytes.NewReader(archiveBuf.Bytes()), int64(archiveBuf.Len()))
	assert.NoError(t, err)
	if err != nil {
		return
	}

	var found *zip.File
	for _, f := range zr.File {
		if f.Name == nameInArchive {
			found = f
			break
		}
	}
	if assert.NotNil(t, found, "archive should contain %q", nameInArchive) {
		rc, err := found.Open()
		assert.NoError(t, err)
		data, err := io.ReadAll(rc)
		assert.NoError(t, err)
		assert.NoError(t, rc.Close())
		assert.Equal(t, content, string(data))
	}
}
