// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"path/filepath"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRepository struct {
	path string
}

func (r *mockRepository) RelativePath() string {
	return r.path
}

func TestMergeBaseNoCommonHistory(t *testing.T) {
	repoDir := filepath.Join(t.TempDir(), "repo.git")
	require.NoError(t, gitcmd.NewCommand("init").AddDynamicArguments(repoDir).Run(t.Context()))
	_, _, runErr := gitcmd.NewCommand("fast-import").WithDir(repoDir).WithStdinBytes([]byte(strings.TrimSpace(`
commit refs/heads/branch1
committer User <user@example.com> 1714310400 +0000
data 12
First commit
M 100644 inline file1.txt
data 12
Hello from 1

commit refs/heads/branch2
committer User <user@example.com> 1714310400 +0000
data 13
Second commit
M 100644 inline file2.txt
data 12
Hello from 2
`))).RunStdString(t.Context())
	require.NoError(t, runErr)
	mergeBase, err := MergeBase(t.Context(), &mockRepository{path: repoDir}, "branch1", "branch2")
	assert.Empty(t, mergeBase)
	assert.ErrorIs(t, err, util.ErrNotExist)
}

func TestRepoGetDivergingCommits(t *testing.T) {
	repo := &mockRepository{path: "repo1_bare"}
	do, err := GetDivergingCommits(t.Context(), repo, "master", "branch2")
	assert.NoError(t, err)
	assert.Equal(t, &DivergeObject{
		Ahead:  1,
		Behind: 5,
	}, do)

	do, err = GetDivergingCommits(t.Context(), repo, "master", "master")
	assert.NoError(t, err)
	assert.Equal(t, &DivergeObject{
		Ahead:  0,
		Behind: 0,
	}, do)

	do, err = GetDivergingCommits(t.Context(), repo, "master", "test")
	assert.NoError(t, err)
	assert.Equal(t, &DivergeObject{
		Ahead:  0,
		Behind: 2,
	}, do)
}

func TestGetCommitIDsBetweenReverse(t *testing.T) {
	repo := &mockRepository{path: "repo1_bare"}

	// tests raw commit IDs
	commitIDs, err := GetCommitIDsBetweenReverse(t.Context(), repo,
		"8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2",
		"ce064814f4a0d337b333e646ece456cd39fab612",
		"",
		100,
	)
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"8006ff9adbf0cb94da7dad9e537e53817f9fa5c0",
		"6fbd69e9823458e6c4a2fc5c0f6bc022b2f2acd1",
		"37991dec2c8e592043f47155ce4808d4580f9123",
		"feaf4ba6bc635fec442f46ddd4512416ec43c2c2",
		"ce064814f4a0d337b333e646ece456cd39fab612",
	}, commitIDs)

	commitIDs, err = GetCommitIDsBetweenReverse(t.Context(), repo,
		"8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2",
		"ce064814f4a0d337b333e646ece456cd39fab612",
		"6fbd69e9823458e6c4a2fc5c0f6bc022b2f2acd1",
		100,
	)
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"37991dec2c8e592043f47155ce4808d4580f9123",
		"feaf4ba6bc635fec442f46ddd4512416ec43c2c2",
		"ce064814f4a0d337b333e646ece456cd39fab612",
	}, commitIDs)

	commitIDs, err = GetCommitIDsBetweenReverse(t.Context(), repo,
		"8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2",
		"ce064814f4a0d337b333e646ece456cd39fab612",
		"",
		3,
	)
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"37991dec2c8e592043f47155ce4808d4580f9123",
		"feaf4ba6bc635fec442f46ddd4512416ec43c2c2",
		"ce064814f4a0d337b333e646ece456cd39fab612",
	}, commitIDs)

	// test branch names instead of raw commit IDs.
	commitIDs, err = GetCommitIDsBetweenReverse(t.Context(), repo,
		"test",
		"master",
		"",
		100,
	)
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"feaf4ba6bc635fec442f46ddd4512416ec43c2c2",
		"ce064814f4a0d337b333e646ece456cd39fab612",
	}, commitIDs)

	// add notref to exclude test
	commitIDs, err = GetCommitIDsBetweenReverse(t.Context(), repo,
		"test",
		"master",
		"test",
		100,
	)
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"feaf4ba6bc635fec442f46ddd4512416ec43c2c2",
		"ce064814f4a0d337b333e646ece456cd39fab612",
	}, commitIDs)
}
