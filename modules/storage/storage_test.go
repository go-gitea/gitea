// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testStorageIterator(t *testing.T, typStr Type, cfg *setting.Storage) {
	l, err := NewStorage(typStr, cfg)
	assert.NoError(t, err)

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
		_, err = l.Save(f[0], strings.NewReader(f[1]), -1)
		assert.NoError(t, err)
	}

	expectedList := map[string][]string{
		"a":           {"a/1.txt"},
		"b":           {"b/1.txt", "b/2.txt", "b/3.txt", "b/x 4.txt"},
		"":            {"a/1.txt", "b/1.txt", "b/2.txt", "b/3.txt", "b/x 4.txt", "ab/1.txt"},
		"/":           {"a/1.txt", "b/1.txt", "b/2.txt", "b/3.txt", "b/x 4.txt", "ab/1.txt"},
		".":           {"a/1.txt", "b/1.txt", "b/2.txt", "b/3.txt", "b/x 4.txt", "ab/1.txt"},
		"a/b/../../a": {"a/1.txt"},
	}
	for dir, expected := range expectedList {
		count := 0
		err = l.IterateObjects(dir, func(path string, f Object) error {
			defer f.Close()
			assert.Contains(t, expected, path)
			count++
			return nil
		})
		assert.NoError(t, err)
		assert.Len(t, expected, count)
	}
}

type expectedServeDirectHeaders struct {
	ContentType        string
	ContentDisposition string
}

func testSingleBlobStorageURLContentTypeAndDisposition(t *testing.T, s ObjectStorage, path, name string, expected expectedServeDirectHeaders, reqParams *ServeDirectOptions) {
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

func testBlobStorageURLContentTypeAndDisposition(t *testing.T, typStr Type, cfg *setting.Storage) {
	s, err := NewStorage(typStr, cfg)
	assert.NoError(t, err)

	testFilename := "test.txt"
	_, err = s.Save(testFilename, strings.NewReader("dummy-content"), -1)
	assert.NoError(t, err)

	testSingleBlobStorageURLContentTypeAndDisposition(t, s, testFilename, "test.txt", expectedServeDirectHeaders{
		ContentType:        "text/plain; charset=utf-8",
		ContentDisposition: `inline; filename=test.txt`,
	}, nil)

	testSingleBlobStorageURLContentTypeAndDisposition(t, s, testFilename, "test.pdf", expectedServeDirectHeaders{
		ContentType:        "application/pdf",
		ContentDisposition: `inline; filename=test.pdf`,
	}, nil)

	testSingleBlobStorageURLContentTypeAndDisposition(t, s, testFilename, "test.wasm", expectedServeDirectHeaders{
		ContentDisposition: `inline; filename=test.wasm`,
	}, nil)

	testSingleBlobStorageURLContentTypeAndDisposition(t, s, testFilename, "test.wasm", expectedServeDirectHeaders{
		ContentType:        "application/wasm",
		ContentDisposition: `inline; filename=test.wasm`,
	}, &ServeDirectOptions{
		ContentType: "application/wasm",
	})
	assert.NoError(t, s.Delete(testFilename))
}
