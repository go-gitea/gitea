// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package assetfs

import (
	"bytes"
	"io/fs"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbed(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDataDir := tmpDir + "/data"
	_ = os.MkdirAll(tmpDataDir+"/foo/bar", 0o755)
	_ = os.WriteFile(tmpDataDir+"/a.txt", []byte("a"), 0o644)
	_ = os.WriteFile(tmpDataDir+"/foo/bar/b.txt", bytes.Repeat([]byte("a"), 1000), 0o644)
	_ = os.WriteFile(tmpDataDir+"/foo/c.txt", []byte("c"), 0o644)
	require.NoError(t, GenerateEmbedBindata(tmpDataDir, tmpDir+"/out.dat"))

	data, err := os.ReadFile(tmpDir + "/out.dat")
	require.NoError(t, err)
	efs := NewEmbeddedFS(data)

	// test a non-existing file
	_, err = fs.ReadFile(efs, "not exist")
	assert.ErrorIs(t, err, fs.ErrNotExist)

	// test a normal file (no compression)
	content, err := fs.ReadFile(efs, "a.txt")
	require.NoError(t, err)
	assert.Equal(t, "a", string(content))
	fi, err := fs.Stat(efs, "a.txt")
	require.NoError(t, err)
	_, ok := fi.(EmbeddedFileInfo).GetGzipContent()
	assert.False(t, ok)

	// test a compressed file
	content, err = fs.ReadFile(efs, "foo/bar/b.txt")
	require.NoError(t, err)
	assert.Equal(t, bytes.Repeat([]byte("a"), 1000), content)
	fi, err = fs.Stat(efs, "foo/bar/b.txt")
	require.NoError(t, err)
	assert.False(t, fi.Mode().IsDir())
	assert.True(t, fi.Mode().IsRegular())
	gzipContent, ok := fi.(EmbeddedFileInfo).GetGzipContent()
	assert.True(t, ok)
	assert.Greater(t, len(gzipContent), 1)
	assert.Less(t, len(gzipContent), 1000)

	// test list root directory
	entries, err := fs.ReadDir(efs, ".")
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.Equal(t, "a.txt", entries[0].Name())
	assert.False(t, entries[0].IsDir())

	// test list subdirectory
	entries, err = fs.ReadDir(efs, "foo")
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "bar", entries[0].Name())
	assert.True(t, entries[0].IsDir())
	assert.Equal(t, "c.txt", entries[1].Name())
	assert.False(t, entries[1].IsDir())

	// test directory mode
	fi, err = fs.Stat(efs, "foo")
	require.NoError(t, err)
	assert.True(t, fi.IsDir())
	assert.True(t, fi.Mode().IsDir())
	assert.False(t, fi.Mode().IsRegular())

	// test httpfs
	hfs := http.FS(efs)
	hf, err := hfs.Open("foo/bar/b.txt")
	require.NoError(t, err)
	hi, err := hf.Stat()
	require.NoError(t, err)
	fiEmbedded, ok := hi.(EmbeddedFileInfo)
	require.True(t, ok)
	gzipContent, ok = fiEmbedded.GetGzipContent()
	assert.True(t, ok)
	assert.Greater(t, len(gzipContent), 1)
	assert.Less(t, len(gzipContent), 1000)

	// test httpfs directory listing
	hf, err = hfs.Open("foo")
	require.NoError(t, err)
	dirs, err := hf.Readdir(1)
	require.NoError(t, err)
	assert.Len(t, dirs, 1)
}
