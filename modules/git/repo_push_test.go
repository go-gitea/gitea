// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gitea.dev/modules/git/gitcmd"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPushForceWithLease(t *testing.T) {
	ctx := t.Context()
	remotePath := filepath.Join(t.TempDir(), "remote.git")
	require.NoError(t, Clone(ctx, filepath.Join(testReposDir, "repo1_bare"), remotePath, CloneRepoOptions{Bare: true}))

	workA := filepath.Join(t.TempDir(), "work-a")
	require.NoError(t, Clone(ctx, remotePath, workA, CloneRepoOptions{Branch: "master"}))
	workB := filepath.Join(t.TempDir(), "work-b")
	require.NoError(t, Clone(ctx, remotePath, workB, CloneRepoOptions{Branch: "master"}))

	testCommitFile(t, workA, "lease-a.txt", "commit-a")
	require.NoError(t, Push(ctx, workA, PushOptions{Remote: remotePath, Branch: "master"}))
	leasedCommitID := testBranchCommitID(t, workA, "master")

	// another push takes over the branch, so the lease of "work-a" becomes stale
	testCommitFile(t, workB, "lease-b.txt", "commit-b")
	require.NoError(t, Push(ctx, workB, PushOptions{Remote: remotePath, Branch: "master", Force: true}))
	remoteCommitID := testBranchCommitID(t, remotePath, "master")

	testCommitFile(t, workA, "lease-c.txt", "commit-c")
	err := Push(ctx, workA, PushOptions{
		Remote:         remotePath,
		Branch:         "master",
		ForceWithLease: fmt.Sprintf("%s:%s", BranchPrefix+"master", leasedCommitID),
	})
	assert.True(t, IsErrPushOutOfDate(err), "expected ErrPushOutOfDate, got %v", err)
	assert.Equal(t, remoteCommitID, testBranchCommitID(t, remotePath, "master"))
}

func testBranchCommitID(t *testing.T, repoPath, branch string) string {
	repo, err := OpenRepositoryLocal(t.Context(), repoPath)
	require.NoError(t, err)
	defer repo.Close()
	commitID, err := repo.GetBranchCommitID(t.Context(), branch)
	require.NoError(t, err)
	return commitID
}

func testCommitFile(t *testing.T, repoPath, filename, message string) {
	require.NoError(t, os.WriteFile(filepath.Join(repoPath, filename), []byte(message), 0o644))
	_, _, err := gitcmd.NewCommand("add").AddDashesAndList(filename).WithDir(repoPath).RunStdString(t.Context())
	require.NoError(t, err)
	_, _, err = gitcmd.NewCommand("commit").
		AddConfig("user.name", "Test").
		AddConfig("user.email", "test@example.com").
		AddOptionValues("-m", message).
		WithDir(repoPath).
		RunStdString(t.Context())
	require.NoError(t, err)
}
