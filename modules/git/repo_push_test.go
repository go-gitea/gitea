// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/git/gitcmd"

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

	createCommit(t, workA, "lease-a.txt", "a", "commit-a")
	require.NoError(t, Push(ctx, workA, PushOptions{Remote: remotePath, Branch: "master"}))

	workARepo, err := OpenRepository(ctx, workA)
	require.NoError(t, err)
	oldCommit, err := workARepo.GetBranchCommit("master")
	workARepo.Close()
	require.NoError(t, err)

	createCommit(t, workB, "lease-b.txt", "b", "commit-b")
	require.NoError(t, Push(ctx, workB, PushOptions{Remote: remotePath, Branch: "master", Force: true}))

	remoteRepo, err := OpenRepository(ctx, remotePath)
	require.NoError(t, err)
	remoteCommit, err := remoteRepo.GetBranchCommit("master")
	remoteRepo.Close()
	require.NoError(t, err)

	createCommit(t, workA, "lease-c.txt", "c", "commit-c")
	err = Push(ctx, workA, PushOptions{
		Remote:         remotePath,
		Branch:         "master",
		ForceWithLease: fmt.Sprintf("%s:%s", BranchPrefix+"master", oldCommit.ID.String()),
	})
	assert.Error(t, err)

	remoteRepo, err = OpenRepository(ctx, remotePath)
	require.NoError(t, err)
	updatedCommit, err := remoteRepo.GetBranchCommit("master")
	remoteRepo.Close()
	require.NoError(t, err)
	assert.Equal(t, remoteCommit.ID.String(), updatedCommit.ID.String())
}

func createCommit(t *testing.T, repoPath, filename, content, message string) {
	fullPath := filepath.Join(repoPath, filename)
	require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
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
