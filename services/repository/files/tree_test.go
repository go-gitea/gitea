// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestGetTreeBySHA(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadRepoCommit(t, ctx)
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	sha := ctx.Repo.Repository.DefaultBranch
	page := 1
	perPage := 10
	ctx.SetPathParam(":id", "1")
	ctx.SetPathParam(":sha", sha)

	tree, err := GetTreeBySHA(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, ctx.PathParam(":sha"), page, perPage, true)
	assert.NoError(t, err)
	expectedTree := &api.GitTreeResponse{
		SHA: "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/trees/65f1bf27bc3bf70f64657658635e66094edbcb4d",
		Entries: []api.GitEntry{
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

	assert.EqualValues(t, expectedTree, tree)
}

func Test_GetTreeList(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx1, _ := contexttest.MockContext(t, "user2/repo1")
	contexttest.LoadRepo(t, ctx1, 1)
	contexttest.LoadRepoCommit(t, ctx1)
	contexttest.LoadUser(t, ctx1, 2)
	contexttest.LoadGitRepo(t, ctx1)
	defer ctx1.Repo.GitRepo.Close()

	refName := git.RefNameFromBranch(ctx1.Repo.Repository.DefaultBranch)

	treeList, err := GetTreeList(ctx1, ctx1.Repo.Repository, "", refName, true)
	assert.NoError(t, err)
	assert.Len(t, treeList, 1)
	assert.EqualValues(t, "README.md", treeList[0].Name)
	assert.EqualValues(t, "README.md", treeList[0].Path)
	assert.True(t, treeList[0].IsFile)
	assert.Empty(t, treeList[0].Children)

	ctx2, _ := contexttest.MockContext(t, "org3/repo3")
	contexttest.LoadRepo(t, ctx2, 3)
	contexttest.LoadRepoCommit(t, ctx2)
	contexttest.LoadUser(t, ctx2, 2)
	contexttest.LoadGitRepo(t, ctx2)
	defer ctx2.Repo.GitRepo.Close()

	refName = git.RefNameFromBranch(ctx2.Repo.Repository.DefaultBranch)

	treeList, err = GetTreeList(ctx2, ctx2.Repo.Repository, "", refName, true)
	assert.NoError(t, err)
	assert.Len(t, treeList, 2)
	assert.EqualValues(t, "README.md", treeList[0].Name)
	assert.EqualValues(t, "README.md", treeList[0].Path)
	assert.True(t, treeList[0].IsFile)
	assert.Empty(t, treeList[0].Children)

	assert.EqualValues(t, "doc", treeList[1].Name)
	assert.EqualValues(t, "doc", treeList[1].Path)
	assert.False(t, treeList[1].IsFile)
	assert.Len(t, treeList[1].Children, 1)

	assert.EqualValues(t, "doc.md", treeList[1].Children[0].Name)
	assert.EqualValues(t, "doc/doc.md", treeList[1].Children[0].Path)
	assert.True(t, treeList[1].Children[0].IsFile)
	assert.Empty(t, treeList[1].Children[0].Children)
}
