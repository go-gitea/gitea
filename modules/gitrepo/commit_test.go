// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"path/filepath"
	"strings"
	"testing"

	"gitea.dev/modules/git/gitcmd"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitsCount(t *testing.T) {
	bareRepo1 := &mockRepository{path: "repo1_bare"}

	commitsCount, err := CommitsCount(t.Context(), bareRepo1,
		CommitsCountOptions{
			Revision: []string{"8006ff9adbf0cb94da7dad9e537e53817f9fa5c0"},
		})

	assert.NoError(t, err)
	assert.Equal(t, int64(3), commitsCount)
}

func TestCommitsCountWithoutBase(t *testing.T) {
	bareRepo1 := &mockRepository{path: "repo1_bare"}

	commitsCount, err := CommitsCount(t.Context(), bareRepo1,
		CommitsCountOptions{
			Not:      "master",
			Revision: []string{"branch1"},
		})

	assert.NoError(t, err)
	assert.Equal(t, int64(2), commitsCount)
}

func TestGetLatestCommitTime(t *testing.T) {
	bareRepo1 := &mockRepository{path: "repo1_bare"}
	lct, err := GetLatestCommitTime(t.Context(), bareRepo1)
	assert.NoError(t, err)
	// Time is Sun Nov 13 16:40:14 2022 +0100
	// which is the time of commit
	// ce064814f4a0d337b333e646ece456cd39fab612 (refs/heads/master)
	assert.EqualValues(t, 1668354014, lct.Unix())
}

// repo7_rename has 2 commits, the first adds a.txt and the second rename a.txt to b.txt

func getRepoRename7(t *testing.T) *mockRepository {
	repoDir := filepath.Join(t.TempDir(), "repo.git")
	require.NoError(t, gitcmd.NewCommand("init").AddDynamicArguments(repoDir).Run(t.Context()))
	_, _, runErr := gitcmd.NewCommand("fast-import").WithDir(repoDir).WithStdinBytes([]byte(strings.TrimSpace(`
blob
mark :1
data 0

reset refs/heads/master
commit refs/heads/master
mark :2
author Chi-Iroh <user@example.com> 1778660718 +0200
committer Chi-Iroh <user@example.com> 1778660718 +0200
data 10
Add a.txt
M 100644 :1 a.txt

commit refs/heads/master
mark :3
author Chi-Iroh <user@example.com> 1778660741 +0200
committer Chi-Iroh <user@example.com> 1778660741 +0200
data 22
Rename a.txt to b.txt
from :2
D a.txt
M 100644 :1 b.txt
	`))).RunStdString(t.Context())
	require.NoError(t, runErr)
	return &mockRepository{path: repoDir}
}

func TestFileCommitsCountWithoutRename(t *testing.T) {
	renameRepo7 := getRepoRename7(t)

	commitsCount, err := CommitsCount(t.Context(), renameRepo7,
		CommitsCountOptions{
			Revision: []string{"HEAD"},
			RelPath:  []string{"b.txt"},
		})

	assert.NoError(t, err)
	assert.Equal(t, int64(1), commitsCount)
}

func TestFileCommitsCountWithRename(t *testing.T) {
	renameRepo7 := getRepoRename7(t)

	commitsCount, err := CommitsCount(t.Context(), renameRepo7,
		CommitsCountOptions{
			Revision: []string{"HEAD"},
			RelPath:  []string{"b.txt"},
		})

	assert.NoError(t, err)
	assert.Equal(t, int64(2), commitsCount)
}
