// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"

	"github.com/stretchr/testify/assert"
)

func TestEditorUtils(t *testing.T) {
	unittest.PrepareTestEnv(t)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	t.Run("getUniquePatchBranchName", func(t *testing.T) {
		branchName := getUniquePatchBranchName(t.Context(), "user2", repo)
		assert.Equal(t, "user2-patch-1", branchName)
	})
	t.Run("getClosestParentWithFiles", func(t *testing.T) {
		gitRepo, _ := gitrepo.OpenRepository(git.DefaultContext, repo)
		defer gitRepo.Close()
		treePath := getClosestParentWithFiles(gitRepo, "sub-home-md-img-check", "docs/foo/bar")
		assert.Equal(t, "docs", treePath)
		treePath = getClosestParentWithFiles(gitRepo, "sub-home-md-img-check", "any/other")
		assert.Empty(t, treePath)
	})
}
