// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"bytes"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
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
		_, err = l.Save(f[0], bytes.NewBufferString(f[1]), -1)
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
