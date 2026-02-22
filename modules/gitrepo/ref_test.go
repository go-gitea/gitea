// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"testing"

	"code.gitea.io/gitea/modules/git"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateRefWithOld(t *testing.T) {
	ctx := t.Context()
	baseRepoPath := "../git/tests/repos/repo1_bare"
	tempDir := t.TempDir()

	require.NoError(t, git.Clone(ctx, baseRepoPath, tempDir, git.CloneRepoOptions{Bare: true}))

	repo := &mockRepository{path: tempDir}
	gitRepo, err := OpenRepository(ctx, repo)
	require.NoError(t, err)
	defer gitRepo.Close()

	masterCommit, err := gitRepo.GetBranchCommit("master")
	require.NoError(t, err)
	branchCommit, err := gitRepo.GetBranchCommit("branch2")
	require.NoError(t, err)
	require.NotEqual(t, masterCommit.ID.String(), branchCommit.ID.String())

	refName := git.BranchPrefix + "master"

	err = UpdateRefWithOld(ctx, repo, refName, branchCommit.ID.String(), masterCommit.ID.String())
	require.NoError(t, err)

	updatedID, err := gitRepo.GetRefCommitID(refName)
	require.NoError(t, err)
	assert.Equal(t, branchCommit.ID.String(), updatedID)

	err = UpdateRefWithOld(ctx, repo, refName, masterCommit.ID.String(), masterCommit.ID.String())
	assert.Error(t, err)

	updatedID, err = gitRepo.GetRefCommitID(refName)
	require.NoError(t, err)
	assert.Equal(t, branchCommit.ID.String(), updatedID)
}
