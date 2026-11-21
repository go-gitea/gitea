// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unittest_test

import (
	"os"
	"path/filepath"
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncDirsRestoresDestination(t *testing.T) {
	srcPath, destPath := t.TempDir(), t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(srcPath, "dir/sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcPath, "dir/file"), []byte("content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(srcPath, "dir/.keep"), nil, 0o644))
	require.NoError(t, unittest.SyncDirs(srcPath, destPath))

	require.NoError(t, os.WriteFile(filepath.Join(destPath, "dir/file"), []byte("modified content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(destPath, "extra"), nil, 0o644))
	require.NoError(t, os.RemoveAll(filepath.Join(destPath, "dir/sub")))
	require.NoError(t, unittest.SyncDirs(srcPath, destPath))

	entries, err := util.ListDirRecursively(destPath, &util.ListDirOptions{IncludeDir: true})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"dir/", "dir/file", "dir/sub/"}, entries)
	content, err := os.ReadFile(filepath.Join(destPath, "dir/file"))
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))
}
