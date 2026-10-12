// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNeedsGarbageCollection(t *testing.T) {
	repoPath := t.TempDir()
	repo := mockRepository(repoPath)
	assert.False(t, NeedsGarbageCollection(repo))

	writeObjectsFile := func(name string) {
		path := filepath.Join(repoPath, "objects", name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, nil, 0o644))
	}
	for _, name := range []string{"pack/pack-a.pack", "pack/pack-b.pack", "pack/pack-b.mtimes", "pack/pack-c.pack", "pack/pack-c.keep"} {
		writeObjectsFile(name)
	}
	assert.False(t, NeedsGarbageCollection(repo))

	writeObjectsFile("ab/cdef")
	assert.True(t, NeedsGarbageCollection(repo))

	require.NoError(t, os.RemoveAll(filepath.Join(repoPath, "objects", "ab")))
	writeObjectsFile("pack/pack-d.pack")
	assert.True(t, NeedsGarbageCollection(repo))
}
