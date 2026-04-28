// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/git/gitcmd"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRepository struct {
	path string
}

func (r *mockRepository) RelativePath() string {
	return r.path
}

func commitRootTree(t *testing.T, repoDir, fileName, content, message string) string {
	t.Helper()

	require.NoError(t, gitcmd.NewCommand("read-tree", "--empty").WithDir(repoDir).Run(t.Context()))

	stdout, _, err := gitcmd.NewCommand("hash-object", "-w", "--stdin").
		WithDir(repoDir).
		WithStdinBytes([]byte(content)).
		RunStdString(t.Context())
	require.NoError(t, err)
	blobSHA := strings.TrimSpace(stdout)

	_, _, err = gitcmd.NewCommand("update-index", "--add", "--replace", "--cacheinfo").
		AddDynamicArguments("100644", blobSHA, fileName).
		WithDir(repoDir).
		RunStdString(t.Context())
	require.NoError(t, err)

	stdout, _, err = gitcmd.NewCommand("write-tree").WithDir(repoDir).RunStdString(t.Context())
	require.NoError(t, err)
	treeSHA := strings.TrimSpace(stdout)

	commitTimeStr := time.Now().Format(time.RFC3339)
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE="+commitTimeStr,
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_COMMITTER_DATE="+commitTimeStr,
	)

	messageBytes := bytes.NewBufferString(message + "\n")
	stdout, _, err = gitcmd.NewCommand("commit-tree").AddDynamicArguments(treeSHA).
		WithEnv(env).
		WithDir(repoDir).
		WithStdinBytes(messageBytes.Bytes()).
		RunStdString(t.Context())
	require.NoError(t, err)

	return strings.TrimSpace(stdout)
}

func TestMergeBaseNoCommonHistory(t *testing.T) {
	repoDir := filepath.Join(t.TempDir(), "repo.git")
	require.NoError(t, gitcmd.NewCommand("init").AddDynamicArguments(repoDir).Run(t.Context()))

	baseCommit := commitRootTree(t, repoDir, "base.txt", "base", "base")
	headCommit := commitRootTree(t, repoDir, "head.txt", "head", "head")

	mergeBase, err := MergeBase(t.Context(), &mockRepository{path: repoDir}, baseCommit, headCommit)
	var noMergeBase ErrNoMergeBase
	assert.Empty(t, mergeBase)
	assert.ErrorAs(t, err, &noMergeBase)
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
