// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"bytes"
	"context"
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
			"a",
			"0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
			"a/0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
		},
		{
			"a",
			"../0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
			"a/0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
		},
		{
			"a",
			"0\\a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
			"a/0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
		},
		{
			"b",
			"a/../0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
			"b/0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
		},
		{
			"b",
			"a\\..\\0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
			"b/0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
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
	l, err := NewLocalStorage(context.Background(), LocalStorageConfig{Path: "testdata/"})
	assert.NoError(t, err)

	test_files := [][]string{
		{"a/1.txt", "a1"},
		{"/a/1.txt", "aa1"},
		{"b/1.txt", "b1"},
		{"b/2.txt", "b2"},
		{"b/3.txt", "b3"},
	}
	for _, f := range test_files {
		_, err = l.Save(f[0], bytes.NewBufferString(f[1]), -1)
		assert.NoError(t, err)
	}

	expected_list := map[string][]string{
		"a": {"a/1.txt", "a/a/1.txt"},
		"b": {"b/1.txt", "b/2.txt", "b/3.txt"},
		"":  {"a/1.txt", "a/a/1.txt", "b/1.txt", "b/2.txt", "b/3.txt"},
		"/": {"a/1.txt", "a/a/1.txt", "b/1.txt", "b/2.txt", "b/3.txt"},
	}
	for dir, expected := range expected_list {
		err = l.IterateObjects(dir, func(path string, f Object) error {
			defer f.Close()
			assert.Contains(t, expected, path)
			return nil
		})
		assert.NoError(t, err)
	}
}
