// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/services/context"

	"github.com/go-enry/go-enry/v2"
)

// TreeList get all files' entries of a repository
func TreeList(ctx *context.Context) {
	_, commit, err := ctx.Repo.GetRefCommit(ctx.FormString("ref"), git.RefTypeBranch, git.RefTypeTag, git.RefTypeCommit)
	if err != nil {
		ctx.ServerError("GetRefCommit", err)
		return
	}
	tree, err := commit.SubTree("/")
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
