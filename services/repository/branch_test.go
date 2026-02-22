// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteBranchReturnsNotFoundWhenMissing(t *testing.T) {
	unittest.PrepareTestEnv(t)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	gitRepo, err := gitrepo.OpenRepository(t.Context(), repo)
	require.NoError(t, err)
	defer gitRepo.Close()

	require.NoError(t, DeleteBranch(t.Context(), doer, repo, gitRepo, "branch2", nil))

	err = DeleteBranch(t.Context(), doer, repo, gitRepo, "branch2", nil)
	assert.True(t, git.IsErrBranchNotExist(err))

	err = DeleteBranch(t.Context(), doer, repo, gitRepo, "branch-does-not-exist", nil)
	assert.True(t, git.IsErrBranchNotExist(err))
}
