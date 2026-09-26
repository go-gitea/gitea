// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"gitea.dev/modules/git/gitrepo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateRefWithOld(t *testing.T) {
	ctx := t.Context()
	repoPath := t.TempDir()
	require.NoError(t, Clone(ctx, testReposDir+"repo1_bare", repoPath, CloneRepoOptions{Bare: true}))

	repo := gitrepo.RepositoryUnmanaged(repoPath)
	masterCommitID := testBranchCommitID(t, repoPath, "master")
	branchCommitID := testBranchCommitID(t, repoPath, "branch2")
	require.NotEqual(t, masterCommitID, branchCommitID)

	require.NoError(t, UpdateRefWithOld(ctx, repo, BranchPrefix+"master", branchCommitID, masterCommitID))
	assert.Equal(t, branchCommitID, testBranchCommitID(t, repoPath, "master"))

	// the old commit ID does not match anymore, so the ref must be left untouched
	assert.Error(t, UpdateRefWithOld(ctx, repo, BranchPrefix+"master", masterCommitID, masterCommitID))
	assert.Equal(t, branchCommitID, testBranchCommitID(t, repoPath, "master"))
}

func TestRefName(t *testing.T) {
	// Test branch names (with and without slash).
	assert.Equal(t, "foo", RefName("refs/heads/foo").BranchName())
	assert.Equal(t, "feature/foo", RefName("refs/heads/feature/foo").BranchName())

	// Test tag names (with and without slash).
	assert.Equal(t, "foo", RefName("refs/tags/foo").TagName())
	assert.Equal(t, "release/foo", RefName("refs/tags/release/foo").TagName())

	// Test pull names
	pullIndex, ok := RefName("refs/pull/1/head").PullIndex()
	assert.True(t, ok)
	assert.EqualValues(t, 1, pullIndex)
	assert.True(t, RefName("refs/pull/1/head").IsPull())
	assert.True(t, RefName("refs/pull/1/merge").IsPull())
	assert.Equal(t, "my/pull", RefName("refs/pull/my/pull/head").ShortName())

	// Test for branch names
	assert.Equal(t, "main", RefName("refs/for/main").ForBranchName())
	assert.Equal(t, "my/branch", RefName("refs/for/my/branch").ForBranchName())

	// Test commit hashes.
	assert.Equal(t, "c0ffee", RefName("c0ffee").ShortName())
}

func TestRefWebLinkPath(t *testing.T) {
	assert.Equal(t, "branch/foo", RefName("refs/heads/foo").RefWebLinkPath())
	assert.Equal(t, "tag/foo", RefName("refs/tags/foo").RefWebLinkPath())
	assert.Equal(t, "commit/c0ffee", RefName("c0ffee").RefWebLinkPath())
}

func TestParseRefSuffix(t *testing.T) {
	cases := []struct {
		ref, name, suffix string
	}{
		{"main", "main", ""},
		{"main^", "main", "^"},
		{"main^2", "main", "^2"},
		{"main~3", "main", "~3"},
		{"main@{yesterday}", "main", "@{yesterday}"},
		{"main~2^", "main", "~2^"},
		{"main^~2", "main", "^~2"},
	}
	for _, c := range cases {
		name, suffix := ParseRefSuffix(c.ref)
		assert.Equal(t, c.name, name, "ref: %s", c.ref)
		assert.Equal(t, c.suffix, suffix, "ref: %s", c.ref)
	}
}
