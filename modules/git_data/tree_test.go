// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git_data

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/sdk/gitea"
)

func TestMain(m *testing.M) {
	models.MainTest(m, filepath.Join("..", ".."))
}

func TestGetTreeBySHA(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	sha := ctx.Repo.Repository.DefaultBranch
	page := 1
	perPage := 10
	ctx.SetParams(":id", "1")
	ctx.SetParams(":sha", sha)

	tree := GetTreeBySHA(ctx.Repo.Repository, ctx.Params("sha"), page, perPage, true)
	expectedTree := &gitea.GitTreeResponse{
		SHA: "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/trees/65f1bf27bc3bf70f64657658635e66094edbcb4d",
		Entries: []gitea.GitEntry{
			{
				Path: "README.md",
				Mode: "100644",
				Type: "blob",
				Size: 30,
				SHA:  "4b4851ad51df6a7d9f25c979345979eaeb5b349f",
				URL:  "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/4b4851ad51df6a7d9f25c979345979eaeb5b349f",
			},
		},
		Truncated:  false,
		Page:       1,
		TotalCount: 1,
	}

	assert.EqualValues(t, tree, expectedTree)
}
