// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	pull_model "code.gitea.io/gitea/models/pull"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/gitdiff"

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

type FileDiffFile struct {
	Name        string
	NameHash    string
	IsSubmodule bool
	IsViewed    bool
	Status      string
}

// transformDiffTreeForUI transforms a DiffTree into a slice of FileDiffFile for UI rendering
// it also takes a map of file names to their viewed state, which is used to mark files as viewed
func transformDiffTreeForUI(diffTree *gitdiff.DiffTree, filesViewedState map[string]pull_model.ViewedState) []FileDiffFile {
	files := make([]FileDiffFile, 0, len(diffTree.Files))

	for _, file := range diffTree.Files {
		nameHash := git.HashFilePathForWebUI(file.HeadPath)
		isSubmodule := file.HeadMode == git.EntryModeCommit
		isViewed := filesViewedState[file.HeadPath] == pull_model.Viewed

		files = append(files, FileDiffFile{
			Name:        file.HeadPath,
			NameHash:    nameHash,
			IsSubmodule: isSubmodule,
			IsViewed:    isViewed,
			Status:      file.Status,
		})
	}

	return files
}
