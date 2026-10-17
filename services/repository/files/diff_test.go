// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestGetDiffPreview(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1")
	ctx.SetPathParam("id", "1")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadRepoCommit(t, ctx)
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	branch := ctx.Repo.Repository.DefaultBranch
	treePath := "README.md"
	oldContent := "# repo1\n\nDescription for repo1"
	content := "# repo1\n\nDescription for repo1\nthis is a new line"

	t.Run("Errors", func(t *testing.T) {
		t.Run("empty repo", func(t *testing.T) {
			diff, err := GetDiffPreview(ctx, &repo_model.Repository{}, branch, treePath, oldContent, content)
			assert.Empty(t, diff)
			assert.EqualError(t, err, "repository does not exist [id: 0, uid: 0, owner_name: , name: ]")
		})

		t.Run("bad branch", func(t *testing.T) {
			badBranch := "bad_branch"
			diff, err := GetDiffPreview(ctx, ctx.Repo.Repository, badBranch, treePath, oldContent, content)
			assert.Empty(t, diff)
			assert.EqualError(t, err, "branch does not exist [name: "+badBranch+"]")
		})

		t.Run("empty treePath", func(t *testing.T) {
			diff, err := GetDiffPreview(ctx, ctx.Repo.Repository, branch, "", oldContent, content)
			assert.Empty(t, diff)
			assert.EqualError(t, err, "path is invalid [path: ]")
		})
	})

	expectedHunk := "@@ -1,3 +1,4 @@\n # repo1\n \n-Description for repo1\n\\ No newline at end of file\n+Description for repo1\n+this is a new line\n\\ No newline at end of file\n"

	t.Run("with given branch", func(t *testing.T) {
		patch, err := GetDiffPreview(ctx, ctx.Repo.Repository, branch, treePath, oldContent, content)
		assert.NoError(t, err)
		assert.Contains(t, patch, expectedHunk)
	})

	t.Run("empty branch, same results", func(t *testing.T) {
		patch, err := GetDiffPreview(ctx, ctx.Repo.Repository, "", treePath, oldContent, content)
		assert.NoError(t, err)
		assert.Contains(t, patch, expectedHunk)
	})
}
