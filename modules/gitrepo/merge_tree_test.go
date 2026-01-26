// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parseMergeTreeOutput(t *testing.T) {
	conflictedOutput := "837480c2773160381cbe6bcce90f7732789b5856\x00options/locale/locale_en-US.ini\x00services/webhook/webhook_test.go\x00"
	treeID, conflictedFiles, err := parseMergeTreeOutput(strings.NewReader(conflictedOutput), 10)
	assert.NoError(t, err)
	assert.Equal(t, "837480c2773160381cbe6bcce90f7732789b5856", treeID)
	assert.Len(t, conflictedFiles, 2)
	assert.Equal(t, "options/locale/locale_en-US.ini", conflictedFiles[0])
	assert.Equal(t, "services/webhook/webhook_test.go", conflictedFiles[1])

	nonConflictedOutput := "837480c2773160381cbe6bcce90f7732789b5856\x00"
	treeID, conflictedFiles, err = parseMergeTreeOutput(strings.NewReader(nonConflictedOutput), 10)
	assert.NoError(t, err)
	assert.Equal(t, "837480c2773160381cbe6bcce90f7732789b5856", treeID)
	assert.Empty(t, conflictedFiles)
}

func prepareRepoDirRenameConflict(t *testing.T) string {
	t.Helper()

	repoDir := filepath.Join(t.TempDir(), "repo-dir-rename-conflict.git")
	repoDir, err := filepath.Abs(repoDir)
	require.NoError(t, err)
	importPath := filepath.Join(test.SetupGiteaRoot(), "modules/git/tests/testdata/repo-dir-rename-conflict.fast-import")

	require.NoError(t, gitcmd.NewCommand("init", "--bare").AddDynamicArguments(repoDir).Run(t.Context()))

	importFile, err := os.Open(importPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = importFile.Close()
	})

	require.NoError(t, gitcmd.NewCommand("fast-import").WithDir(repoDir).WithStdinCopy(importFile).Run(t.Context()))

	return repoDir
}

func TestMergeTreeDirectoryRenameConflictWithoutFiles(t *testing.T) {
	if !git.DefaultFeatures().SupportGitMergeTree {
		t.Skip("git merge-tree not supported")
	}

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
