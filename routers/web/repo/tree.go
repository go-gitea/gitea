// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
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
	dir := ctx.PathParam("*")
	ref := ctx.FormTrim("ref")
	recursive := ctx.FormBool("recursive")

	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("RepositoryFromContextOrOpen", err)
		return
	}
	defer closer.Close()

	refName := gitRepo.UnstableGuessRefByShortName(ref)

	results, err := files_service.GetTreeList(ctx, ctx.Repo.Repository, dir, refName, recursive)
	if err != nil {
		ctx.ServerError("GetTreeList", err)
		return
	}

	ctx.JSON(http.StatusOK, results)
}
