// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/services/context"
	files_service "code.gitea.io/gitea/services/repository/files"

	"github.com/go-enry/go-enry/v2"
)

// TreeList get all files' entries of a repository
func TreeList(ctx *context.Context) {
	tree, err := ctx.Repo.Commit.SubTree("/")
	if err != nil {
		ctx.ServerError("Repo.Commit.SubTree", err)
		return
	}

	entries, err := tree.ListEntriesRecursiveFast()
	if err != nil {
		ctx.ServerError("ListEntriesRecursiveFast", err)
		return
	}
	entries.CustomSort(base.NaturalSortLess)

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !isExcludedEntry(entry) {
			files = append(files, entry.Name())
		}
	}
	ctx.JSON(http.StatusOK, files)
}

func isExcludedEntry(entry *git.TreeEntry) bool {
	if entry.IsDir() {
		return true
	}

	if entry.IsSubModule() {
		return true
	}

	if enry.IsVendor(entry.Name()) {
		return true
	}

	return false
}

func Tree(ctx *context.Context) {
	recursive := ctx.FormBool("recursive")

	if ctx.Repo.RefFullName == "" {
		ctx.Error(http.StatusBadRequest, "RefFullName", "ref_name is invalid")
		return
	}

	var results []*files_service.TreeViewNode
	var err error
	if !recursive {
		results, err = files_service.GetTreeList(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, ctx.Repo.TreePath, ctx.Repo.RefFullName, false)
	} else {
		results, err = files_service.GetTreeInformation(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, ctx.Repo.TreePath, ctx.Repo.RefFullName)
	}
	if err != nil {
		ctx.ServerError("GetTreeInformation", err)
		return
	}
	ctx.JSON(http.StatusOK, results)
}
