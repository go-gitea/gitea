// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"os"
	"path/filepath"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestDeleteRepoFile(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadRepoCommit(t, ctx)
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	// Remove hooks to avoid "gitea: no such file or directory" error
	repoPath := repo_model.RepoPath(ctx.Repo.Repository.OwnerName, ctx.Repo.Repository.Name)
	assert.NoError(t, os.RemoveAll(filepath.Join(repoPath, "hooks")))

	t.Run("DeleteRoot", func(t *testing.T) {
		_, err := DeleteRepoFile(ctx, ctx.Repo.Repository, ctx.Doer, &DeleteRepoFileOptions{
			TreePath:  "",
			OldBranch: "master",
			NewBranch: "master",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path cannot be empty")
	})

	t.Run("DeleteFile", func(t *testing.T) {
		// README.md exists in repo1
		_, err := DeleteRepoFile(ctx, ctx.Repo.Repository, ctx.Doer, &DeleteRepoFileOptions{
			TreePath:  "README.md",
			OldBranch: "master",
			NewBranch: "master",
			Message:   "Delete README.md",
		})
		assert.NoError(t, err)
	})
}
