// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"gitea.dev/modules/git/gitcmd"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prepareRepoDirRenameConflict(t *testing.T) string {
	repoDir := filepath.Join(t.TempDir(), "repo-dir-rename-conflict.git")
	require.NoError(t, gitcmd.NewCommand("init", "--bare").AddDynamicArguments(repoDir).RunWithStderr(t.Context()))
	stdin := `blob
mark :1
data 2
b

blob
mark :2
data 2
c

reset refs/heads/master
commit refs/heads/master
mark :3
author test <test@example.com> 1769202331 -0800
committer test <test@example.com> 1769202331 -0800
data 2
O
M 100644 :1 z/b
M 100644 :2 z/c

commit refs/heads/split
mark :4
author test <test@example.com> 1769202336 -0800
committer test <test@example.com> 1769202336 -0800
data 2
A
from :3
M 100644 :2 w/c
M 100644 :1 y/b
D z/b
D z/c

blob
mark :5
data 2
d

commit refs/heads/add
mark :6
author test <test@example.com> 1769202342 -0800
committer test <test@example.com> 1769202342 -0800
data 2
B
from :3
M 100644 :5 z/d
`
	require.NoError(t, gitcmd.NewCommand("fast-import").WithDir(repoDir).WithStdinBytes([]byte(stdin)).RunWithStderr(t.Context()))
	return repoDir
}

func TestMergeTreeDirectoryRenameConflictWithoutFiles(t *testing.T) {
	repoDir := prepareRepoDirRenameConflict(t)
	require.DirExists(t, repoDir)
	repo := mockRepository(repoDir)

	mergeBase, err := MergeBase(t.Context(), repo, "add", "split")
	require.NoError(t, err)

	treeID, conflicted, conflictedFiles, err := MergeTree(t.Context(), repo, "add", "split", mergeBase)
	require.NoError(t, err)
	assert.True(t, conflicted)
	assert.Empty(t, conflictedFiles)
	assert.Equal(t, "5e3dd4cfc5b11e278a35b2daa83b7274175e3ab1", treeID)
}

func TestParseMergeTreeConflictInfo(t *testing.T) {
	info, stage, ok := parseMergeTreeConflictInfo("100644 0123456789abcdef0123456789abcdef01234567 2\tdir/some file.txt")
	assert.True(t, ok)
	assert.Equal(t, 2, stage)
	assert.Equal(t, "dir/some file.txt", info.path)
	assert.Equal(t, MergeTreeConflictStage{Mode: "100644", ObjectID: "0123456789abcdef0123456789abcdef01234567"}, info.stage)

	for _, line := range []string{"", "1\tfile", "100644 abc 4\tfile", "100644 abc x\tfile", "100644 abc 1", "100644 abc 1\t"} {
		_, _, ok = parseMergeTreeConflictInfo(line)
		assert.False(t, ok, line)
	}
}
