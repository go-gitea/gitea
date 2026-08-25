// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectStoragePath(t *testing.T) {
	base := ""
	assert.Empty(t, buildObjectStorePath(base, "/"))
	assert.Empty(t, buildObjectStorePath(base, "."))
	assert.Equal(t, "a", buildObjectStorePath(base, "/a"))
	assert.Equal(t, "a/b", buildObjectStorePath(base, "/a/b/"))
	assert.Empty(t, buildObjectStorePathPrefix(base, ""))
	assert.Equal(t, "a/", buildObjectStorePathPrefix(base, "/a/"))

	base = "/"
	assert.Empty(t, buildObjectStorePath(base, "/"))
	assert.Empty(t, buildObjectStorePath(base, "."))
	assert.Equal(t, "a", buildObjectStorePath(base, "/a"))
	assert.Equal(t, "a/b", buildObjectStorePath(base, "/a/b/"))
	assert.Empty(t, buildObjectStorePathPrefix(base, ""))
	assert.Equal(t, "a/", buildObjectStorePathPrefix(base, "/a/"))

	base = "/base"
	assert.Equal(t, "base", buildObjectStorePath(base, "/"))
	assert.Equal(t, "base", buildObjectStorePath(base, "."))
	assert.Equal(t, "base/a", buildObjectStorePath(base, "/a"))
	assert.Equal(t, "base/a/b", buildObjectStorePath(base, "/a/b/"))
	assert.Equal(t, "base/", buildObjectStorePathPrefix(base, ""))
	assert.Equal(t, "base/a/", buildObjectStorePathPrefix(base, "/a/"))

	base = "/base/"
	assert.Equal(t, "base", buildObjectStorePath(base, "/"))
	assert.Equal(t, "base", buildObjectStorePath(base, "."))
	assert.Equal(t, "base/a", buildObjectStorePath(base, "/a"))
	assert.Equal(t, "base/a/b", buildObjectStorePath(base, "/a/b/"))
	assert.Equal(t, "base/", buildObjectStorePathPrefix(base, ""))
	assert.Equal(t, "base/a/", buildObjectStorePathPrefix(base, "/a/"))
}

func testStorageIterator(t *testing.T, objStore ObjectStorage) {
	testFiles := [][]string{
		{"a/1.txt", "a1"},
		{"/a/1.txt", "aa1"}, // same as above, but with leading slash that will be trim
		{"ab/1.txt", "ab1"},
		{"b/1.txt", "b1"},
		{"b/2.txt", "b2"},
		{"b/3.txt", "b3"},
		{"b/x 4.txt", "bx4"},
	}
	for _, f := range testFiles {
		_, err := objStore.Save(f[0], strings.NewReader(f[1]), -1)
		assert.NoError(t, err)
	}
	defer func() {
		for _, f := range testFiles {
			_ = objStore.Delete(f[0])
		}
	}()

	expectedList := map[string][]string{
		"a":           {"a/1.txt"},
		"a/":          {"a/1.txt"},
		"/a/":         {"a/1.txt"},
		"b":           {"b/1.txt", "b/2.txt", "b/3.txt", "b/x 4.txt"},
		"":            {"a/1.txt", "b/1.txt", "b/2.txt", "b/3.txt", "b/x 4.txt", "ab/1.txt"},
		"/":           {"a/1.txt", "b/1.txt", "b/2.txt", "b/3.txt", "b/x 4.txt", "ab/1.txt"},
		".":           {"a/1.txt", "b/1.txt", "b/2.txt", "b/3.txt", "b/x 4.txt", "ab/1.txt"},
		"a/b/../../a": {"a/1.txt"},
	}
	for dir, expected := range expectedList {
		count := 0
		err := objStore.IterateObjects(dir, func(path string, f Object) error {
			content, err := io.ReadAll(f)
			assert.NoError(t, err)
			assert.NotEmpty(t, content)
			assert.Contains(t, expected, path)
			count++
			return nil
		})
		assert.NoError(t, err)
		assert.Len(t, expected, count)
	}
}

func testStorageURLContentTypeAndDisposition(t *testing.T, objStore ObjectStorage) {
	type expectedServeDirectHeaders struct {
		ContentType        string
		ContentDisposition string
	}
	test := func(t *testing.T, s ObjectStorage, path, name string, expected expectedServeDirectHeaders, reqParams *ServeDirectOptions) {
		u, err := s.ServeDirectURL(path, name, http.MethodGet, reqParams)
		require.NoError(t, err)
		resp, err := http.Get(u.String())
		require.NoError(t, err)
		defer resp.Body.Close()
		if expected.ContentType != "" {
			assert.Equal(t, expected.ContentType, resp.Header.Get("Content-Type"))
		}
		if expected.ContentDisposition != "" {
			assert.Equal(t, expected.ContentDisposition, resp.Header.Get("Content-Disposition"))
		}
	}

	testFilename := "test.txt"
	_, err := objStore.Save(testFilename, strings.NewReader("dummy-content"), -1)
	assert.NoError(t, err)

	test(t, objStore, testFilename, "test.txt", expectedServeDirectHeaders{
		ContentType:        "text/plain; charset=utf-8",
		ContentDisposition: `inline; filename=test.txt`,
	}, nil)

	test(t, objStore, testFilename, "test.pdf", expectedServeDirectHeaders{
		ContentType:        "application/pdf",
		ContentDisposition: `inline; filename=test.pdf`,
	}, nil)

	test(t, objStore, testFilename, "test.wasm", expectedServeDirectHeaders{
		ContentDisposition: `inline; filename=test.wasm`,
	}, nil)

	test(t, objStore, testFilename, "test.wasm", expectedServeDirectHeaders{
		ContentType:        "application/wasm",
		ContentDisposition: `inline; filename=test.wasm`,
	}, &ServeDirectOptions{
		ContentType: "application/wasm",
	})
	assert.NoError(t, objStore.Delete(testFilename))
}

func testStorageGeneral(t *testing.T, objStore ObjectStorage) {
	t.Run("StorageIterator", func(t *testing.T) { testStorageIterator(t, objStore) })

	if _, ok := objStore.(*LocalStorage); ok {
		t.Skipf("Skipping tests for local storage")
	}
	t.Run("StorageURLContentTypeAndDisposition", func(t *testing.T) { testStorageURLContentTypeAndDisposition(t, objStore) })
}
