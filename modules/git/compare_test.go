// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"strings"
	"testing"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	mergeBase, err := MergeBase(t.Context(), mockRepository(repoDir), "branch1", "branch2")
	assert.Empty(t, mergeBase)
	assert.ErrorIs(t, err, util.ErrNotExist)
}

func TestRepoGetDivergingCommits(t *testing.T) {
	repo := mockRepository("repo1_bare")
	for _, testcase := range []struct {
		branch   string
		expected DivergeObject
	}{
		{"branch2", DivergeObject{Ahead: 1, Behind: 5}},
		{"master", DivergeObject{Ahead: 0, Behind: 0}},
		{"test", DivergeObject{Ahead: 0, Behind: 2}},
	} {
		do, err := GetDivergingCommits(t.Context(), repo, "master", testcase.branch)
		assert.NoError(t, err)
		assert.Equal(t, &testcase.expected, do)
	}
}

func TestGetCommitIDsBetweenReverse(t *testing.T) {
	repo := mockRepository("repo1_bare")
	startRef := "8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2"
	endRef := "ce064814f4a0d337b333e646ece456cd39fab612"
	expectedCommitIDs := []string{
		"8006ff9adbf0cb94da7dad9e537e53817f9fa5c0",
		"6fbd69e9823458e6c4a2fc5c0f6bc022b2f2acd1",
		"37991dec2c8e592043f47155ce4808d4580f9123",
		"feaf4ba6bc635fec442f46ddd4512416ec43c2c2",
		"ce064814f4a0d337b333e646ece456cd39fab612",
	}
	for _, testcase := range []struct {
		startRef, endRef, notRef string
		limit                    int
		expected                 []string
	}{
		// tests raw commit IDs
		{startRef, endRef, "", 100, expectedCommitIDs},
		{startRef, endRef, expectedCommitIDs[1], 100, expectedCommitIDs[2:]},
		{startRef, endRef, "", 3, expectedCommitIDs[2:]},
		// test branch names instead of raw commit IDs.
		{"test", "master", "", 100, expectedCommitIDs[3:]},
		// add notref to exclude test
		{"test", "master", "test", 100, expectedCommitIDs[3:]},
	} {
		commitIDs, err := GetCommitIDsBetweenReverse(t.Context(), repo, testcase.startRef, testcase.endRef, testcase.notRef, testcase.limit)
		assert.NoError(t, err)
		assert.Equal(t, testcase.expected, commitIDs)
	}
}
