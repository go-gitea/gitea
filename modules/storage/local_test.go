// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"os"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

			assert.Equal(t, k.expected, l.buildLocalPath(k.path))
		})
	}
}

func TestLocalStorageDelete(t *testing.T) {
	rootDir := t.TempDir()
	st, err := NewLocalStorage(t.Context(), &setting.Storage{Path: rootDir})
	require.NoError(t, err)

	assertExists := func(t *testing.T, path string, exists bool) {
		_, err = os.Stat(rootDir + "/" + path)
		if exists {
			require.NoError(t, err)
		} else {
			require.ErrorIs(t, err, os.ErrNotExist)
		}
	}

	_, err = st.Save("dir/sub1/1-a.txt", strings.NewReader(""), -1)
	require.NoError(t, err)
	_, err = st.Save("dir/sub1/1-b.txt", strings.NewReader(""), -1)
	require.NoError(t, err)
	_, err = st.Save("dir/sub2/2-a.txt", strings.NewReader(""), -1)
	require.NoError(t, err)

	assertExists(t, "dir/sub1/1-a.txt", true)
	assertExists(t, "dir/sub1/1-b.txt", true)
	assertExists(t, "dir/sub2/2-a.txt", true)

	require.NoError(t, st.Delete("dir/sub1/1-a.txt"))
	assertExists(t, "dir/sub1", true)
	assertExists(t, "dir/sub1/1-a.txt", false)
	assertExists(t, "dir/sub1/1-b.txt", true)
	assertExists(t, "dir/sub2/2-a.txt", true)

	require.NoError(t, st.Delete("dir/sub1/1-b.txt"))
	assertExists(t, ".", true)
	assertExists(t, "dir/sub1", false)
	assertExists(t, "dir/sub1/1-a.txt", false)
	assertExists(t, "dir/sub1/1-b.txt", false)
	assertExists(t, "dir/sub2/2-a.txt", true)

	require.NoError(t, st.Delete("dir/sub2/2-a.txt"))
	assertExists(t, ".", true)
	assertExists(t, "dir", false)
}

func TestLocalStorageIterator(t *testing.T) {
	testStorageIterator(t, setting.LocalStorageType, &setting.Storage{Path: t.TempDir()})
}
