// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"html/template"
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
	ctx.SetPathParam("id", "1")
	ctx.SetPathParam("sha", sha)

	tree, err := GetTreeBySHA(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, ctx.PathParam("sha"), page, perPage, true)
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

func TestGetTreeViewNodes(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1")
	ctx.Repo.RefFullName = git.RefNameFromBranch("sub-home-md-img-check")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadRepoCommit(t, ctx)
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	treeNodes, err := GetTreeViewNodes(ctx, ctx.Repo.Commit, "", "")
	assert.NoError(t, err)
	assert.Equal(t, []*TreeViewNode{
		{
			EntryName: "docs",
			EntryMode: "tree",
			FileIcon:  template.HTML(`<svg id="svg-mfi-folder-docs" class="svg git-entry-icon octicon-file-directory-fill" width="16" height="16" aria-hidden="true" viewBox='0 0 32 32'><path fill='#0277bd' d='m13.844 7.536-1.288-1.072A2 2 0 0 0 11.276 6H4a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h24a2 2 0 0 0 2-2V10a2 2 0 0 0-2-2H15.124a2 2 0 0 1-1.28-.464'/><path fill='#b3e5fc' d='M24 10h-7a1 1 0 0 0-1 1v16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V16Zm0 16h-6v-2h6Zm4-4H18v-2h10Zm-4.828-5.172V12L28 16.828Z'/></svg>`),
			FullPath:  "docs",
		},
	}, treeNodes)

	treeNodes, err = GetTreeViewNodes(ctx, ctx.Repo.Commit, "", "docs/README.md")
	assert.NoError(t, err)
	assert.Equal(t, []*TreeViewNode{
		{
			EntryName: "docs",
			EntryMode: "tree",
			FileIcon:  template.HTML(`<svg class="svg git-entry-icon octicon-file-directory-fill" width="16" height="16" aria-hidden="true"><use xlink:href="#svg-mfi-folder-docs"></use></svg>`),
			FullPath:  "docs",
			Children: []*TreeViewNode{
				{
					EntryName: "README.md",
					EntryMode: "blob",
					FileIcon:  template.HTML(`<svg id="svg-mfi-readme" class="svg git-entry-icon octicon-file" width="16" height="16" aria-hidden="true" fill='none' viewBox='0 0 16 16'><path d='M0 0h24v24H0z'/><path fill='#42a5f5' d='M8 1C4.136 1 1 4.136 1 8s3.136 7 7 7 7-3.136 7-7-3.136-7-7-7m1 11H7V7.5h2zm0-6H7V4h2z'/></svg>`),
					FullPath:  "docs/README.md",
				},
			},
		},
	}, treeNodes)

	treeNodes, err = GetTreeViewNodes(ctx, ctx.Repo.Commit, "docs", "README.md")
	assert.NoError(t, err)
	assert.Equal(t, []*TreeViewNode{
		{
			EntryName: "README.md",
			EntryMode: "blob",
			FileIcon:  template.HTML(`<svg class="svg git-entry-icon octicon-file" width="16" height="16" aria-hidden="true"><use xlink:href="#svg-mfi-readme"></use></svg>`),
			FullPath:  "docs/README.md",
		},
	}, treeNodes)
}
