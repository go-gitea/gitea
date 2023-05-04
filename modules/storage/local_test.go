// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildLocalPath(t *testing.T) {
	kases := []struct {
		localDir string
		path     string
		expected string
	}{
		{
			"/a",
			"0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
			"/a/0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
		},
		{
			"/a",
			"../0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
			"/a/0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
		},
		{
			"/a",
			"0\\a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
			"/a/0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
		},
		{
			"/b",
			"a/../0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
			"/b/0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
		},
		{
			"/b",
			"a\\..\\0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
			"/b/0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
		},
	}

	for _, k := range kases {
		t.Run(k.path, func(t *testing.T) {
			l := LocalStorage{dir: k.localDir}

			assert.EqualValues(t, k.expected, l.buildLocalPath(k.path))
		})
	}
}

func TestLocalStorageIterator(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "TestLocalStorageIteratorTestDir")
	l, err := NewLocalStorage(context.Background(), LocalStorageConfig{Path: dir})
	assert.NoError(t, err)

	testFiles := [][]string{
		{"a/1.txt", "a1"},
		{"/a/1.txt", "aa1"}, // same as above, but with leading slash that will be trim
		{"b/1.txt", "b1"},
		{"b/2.txt", "b2"},
		{"b/3.txt", "b3"},
		{"b/x 4.txt", "bx4"},
	}
	for _, f := range testFiles {
		_, err = l.Save(f[0], bytes.NewBufferString(f[1]), -1)
		assert.NoError(t, err)
	}

	expectedList := map[string][]string{
		"a":           {"a/1.txt"},
		"b":           {"b/1.txt", "b/2.txt", "b/3.txt", "b/x 4.txt"},
		"":            {"a/1.txt", "b/1.txt", "b/2.txt", "b/3.txt", "b/x 4.txt"},
		"/":           {"a/1.txt", "b/1.txt", "b/2.txt", "b/3.txt", "b/x 4.txt"},
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
