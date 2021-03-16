// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"

	_ "code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/context"
)

const (
	tplFindFiles base.TplName = "repo/find/files"
)

// render the page to find repository files
func FindFiles(ctx *context.Context) {
  ctx.Data["Title"] = ctx.Tr("repo.find")
	ctx.Data["PageIsFindFiles"] = true
	ctx.Data["PageIsViewCode"] = true

	fmt.Printf("ctx.Repo.RepoLink: %v\n", ctx.Repo.RepoLink)
	branchLink := ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	treeLink := branchLink

	if len(ctx.Repo.TreePath) > 0 {
		treeLink += "/" + ctx.Repo.TreePath
	}

	// Get current entry user currently looking at.
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		ctx.NotFoundOrServerError("Repo.Commit.GetTreeEntryByPath", git.IsErrNotExist, err)
		return
	}

	if entry.IsDir() {
		renderDirectory(ctx, treeLink)
	} else {
		ctx.Data["Files"] = make([]interface{}, 0)
	}

	if ctx.Written() {
		return
	}

	fmt.Printf("ctx.files: %v\n", ctx.Data["Files"])

	ctx.Data["RepoLink"] = ctx.Repo.RepoLink
	ctx.Data["RepoName"] = ctx.Repo.Repository.Name
	ctx.Data["TreeLink"] = treeLink

	ctx.HTML(200, tplFindFiles)
}