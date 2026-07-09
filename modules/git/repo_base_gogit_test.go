// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenRepositoryRepairsOrphanPack reproduces issue #38359: a packfile whose ".idx"
// is missing (e.g. on Windows, git's repack cleanup deletes the ".idx" but cannot delete
// the ".pack" that the gogit storage keeps open) makes the whole repository unreadable with
// "packfile not found". Opening the repository should regenerate the missing index instead.
func TestOpenRepositoryRepairsOrphanPack(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.CopyFS(tmpDir, os.DirFS(filepath.Join(testReposDir, "repo5_pulls"))))

	// c83380d... only lives inside the packfile, so reading it requires the pack index.
	const packedCommit = "c83380d7056593c51a699d12b9c00627bd5743e9"
	packIdx := filepath.Join(tmpDir, "objects", "pack", "pack-81423f591973f5d9dab89cc45afa1c544448133e.idx")
	require.FileExists(t, packIdx)
	require.NoError(t, os.Remove(packIdx)) // turn the pack into an orphan

	repo, err := OpenRepository(t.Context(), tmpDir)
	require.NoError(t, err)
	defer repo.Close()

	commit, err := repo.GetCommit(packedCommit)
	require.NoError(t, err)
	assert.Equal(t, packedCommit, commit.ID.String())

	// the missing index must have been regenerated while opening the repository
	assert.FileExists(t, packIdx)
}
