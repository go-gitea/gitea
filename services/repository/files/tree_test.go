// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/fileicon"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/test"
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
	defer test.MockVariableValue(&setting.UI.FileIconTheme, "basic")()
	renderUtils := templates.NewRenderUtils(reqctx.FromContext(ctx))

	treeNodes, err := GetTreeViewNodes(ctx, ctx.Repo.Commit, "", "")
	assert.NoError(t, err)
	assert.Equal(t, []*TreeViewNode{
		{
			EntryName: "docs",
			EntryMode: "tree",
			FileIcon: renderUtils.RenderFileIcon(&fileicon.FileIcon{
				Name:      "docs",
				EntryMode: git.EntryModeTree,
			}),
			FullPath: "docs",
		},
	}, treeNodes)

	treeNodes, err = GetTreeViewNodes(ctx, ctx.Repo.Commit, "", "docs/README.md")
	assert.NoError(t, err)
	assert.Equal(t, []*TreeViewNode{
		{
			EntryName: "docs",
			EntryMode: "tree",
			FileIcon: renderUtils.RenderFileIcon(&fileicon.FileIcon{
				Name:      "docs",
				EntryMode: git.EntryModeTree,
			}),
			FullPath: "docs",
			Children: []*TreeViewNode{
				{
					EntryName: "README.md",
					EntryMode: "blob",
					FileIcon: renderUtils.RenderFileIcon(&fileicon.FileIcon{
						Name:      "README.md",
						EntryMode: git.EntryModeBlob,
					}),
					FullPath: "docs/README.md",
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
			FileIcon: renderUtils.RenderFileIcon(&fileicon.FileIcon{
				Name:      "README.md",
				EntryMode: git.EntryModeBlob,
			}),
			FullPath: "docs/README.md",
		},
	}, treeNodes)
}
