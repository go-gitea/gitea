// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/setting"

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
	testStorageIterator(t, setting.LocalStorageType, &setting.Storage{Path: dir})
}
