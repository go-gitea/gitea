// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prepareRepoDirRenameConflict(t *testing.T) string {
	t.Helper()

	repoDir := filepath.Join(t.TempDir(), "repo-dir-rename-conflict.git")
	repoDir, err := filepath.Abs(repoDir)
	require.NoError(t, err)

	require.NoError(t, gitcmd.NewCommand("init", "--bare").AddDynamicArguments(repoDir).Run(t.Context()))

	importPath := filepath.Join(test.SetupGiteaRoot(), "modules/git/tests/testdata/repo-dir-rename-conflict.fast-import")
	importFile, err := os.ReadFile(importPath)
	require.NoError(t, err)
	require.NoError(t, gitcmd.NewCommand("fast-import").WithDir(repoDir).WithStdinBytes(importFile).Run(t.Context()))

	return repoDir
}

func TestMergeTreeDirectoryRenameConflictWithoutFiles(t *testing.T) {
	repoDir := prepareRepoDirRenameConflict(t)
	require.DirExists(t, repoDir)
	repo := &mockRepository{path: repoDir}

	mergeBase, err := MergeBase(t.Context(), repo, "add", "split")
	require.NoError(t, err)

	treeID, conflicted, conflictedFiles, err := MergeTree(t.Context(), repo, "add", "split", mergeBase)
	require.NoError(t, err)
	assert.True(t, conflicted)
	assert.Empty(t, conflictedFiles)
	assert.Equal(t, "5e3dd4cfc5b11e278a35b2daa83b7274175e3ab1", treeID)
}
