// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	_ "code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
)

const (
	tplFindFiles base.TplName = "repo/find/files"
)

// render the page to find repository files
func FindFiles(ctx *context.Context) {
	ctx.Data["PageIsFindFiles"] = true
	ctx.Data["PageIsViewCode"] = true

	branchLink := ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	treeLink := branchLink

	if len(ctx.Repo.TreePath) > 0 {
		treeLink += "/" + ctx.Repo.TreePath
	}

	renderFiles(ctx, treeLink)

	if ctx.Written() {
		return
	}

	ctx.Data["RepoLink"] = ctx.Repo.RepoLink
	ctx.Data["RepoName"] = ctx.Repo.Repository.Name
	ctx.Data["TreeLink"] = treeLink

	ctx.HTML(200, tplFindFiles)
}

func renderFiles(ctx *context.Context, treeLink string) {
	tree, err := ctx.Repo.Commit.SubTree(ctx.Repo.TreePath)
	if err != nil {
		ctx.NotFoundOrServerError("Repo.Commit.SubTree", git.IsErrNotExist, err)
		return
	}

	entries, err := tree.ListEntriesRecursive()
	if err != nil {
		ctx.ServerError("ListEntries", err)
		return
	}
	entries.CustomSort(base.NaturalSortLess)

	var fileEntries []*git.TreeEntry
	for _, entry := range entries {
		if !entry.IsDir() && !entry.IsSubModule() {
			fileEntries = append(fileEntries, entry)
		}
	}
	ctx.Data["Files"] = fileEntries
}
