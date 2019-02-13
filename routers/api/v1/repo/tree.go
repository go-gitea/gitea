// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/sdk/gitea"
)

// GetTree get the tree of a repository.
func GetTree(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/trees/{sha} repository GetTree
	// ---
	// summary: Gets the tree of a repository.
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: sha
	//   in: path
	//   description: sha of the commit
	//   type: string
	//   required: true
	// - name: recursive
	//   in: query
	//   description: show all directories and files
	//   required: false
	//   type: boolean
	// - name: page
	//   in: query
	//   description: page number; the 'truncated' field in the response will be true if there are still more items after this page, false if the last page
	//   required: false
	//   type: integer
	// - name: per_page
	//   in: query
	//   description: number of items per page; default is 1000 or what is set in app.ini as DEFAULT_GIT_TREES_PER_PAGE
	//   required: false
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/GitTreeResponse"
	sha := ctx.Params("sha")
	if len(sha) == 0 {
		ctx.Error(400, "", "sha not provided")
		return
	}
	tree := GetTreeBySHA(ctx, sha)
	if tree != nil {
		ctx.JSON(200, tree)
	} else {
		ctx.Error(400, "", "sha invalid")
	}
}

// GetTreeBySHA get the GitTreeResponse of a repository using a sha hash.
func GetTreeBySHA(ctx *context.APIContext, sha string) *gitea.GitTreeResponse {
	gitTree, err := ctx.Repo.GitRepo.GetTree(sha)
	if err != nil || gitTree == nil {
		return nil
	}
	tree := new(gitea.GitTreeResponse)
	repoID := strings.TrimRight(setting.AppURL, "/") + "/api/v1/repos/" + ctx.Repo.Repository.Owner.Name + "/" + ctx.Repo.Repository.Name
	tree.SHA = gitTree.ID.String()
	tree.URL = repoID + "/git/trees/" + tree.SHA
	var entries git.Entries
	if ctx.QueryBool("recursive") {
		entries, err = gitTree.ListEntriesRecursive()
	} else {
		entries, err = gitTree.ListEntries()
	}
	if err != nil {
		return tree
	}
	repoIDLen := len(repoID)

	// 51 is len(sha1) + len("/git/blobs/"). 40 + 11.
	blobURL := make([]byte, repoIDLen+51)
	copy(blobURL[:], repoID)
	copy(blobURL[repoIDLen:], "/git/blobs/")

	// 51 is len(sha1) + len("/git/trees/"). 40 + 11.
	treeURL := make([]byte, repoIDLen+51)
	copy(treeURL[:], repoID)
	copy(treeURL[repoIDLen:], "/git/trees/")

	// 40 is the size of the sha1 hash in hexadecimal format.
	copyPos := len(treeURL) - 40

	page := ctx.QueryInt("page")
	perPage := ctx.QueryInt("per_page")
	if perPage <= 0 || perPage > setting.API.DefaultGitTreesPerPage {
		perPage = setting.API.DefaultGitTreesPerPage
	}
	if page <= 0 {
		page = 1
	}
	tree.Page = page
	tree.TotalCount = len(entries)
	rangeStart := perPage * (page - 1)
	if rangeStart >= len(entries) {
		return tree
	}
	var rangeEnd int
	if len(entries) > perPage {
		tree.Truncated = true
	}
	if rangeStart+perPage < len(entries) {
		rangeEnd = rangeStart + perPage
	} else {
		rangeEnd = len(entries)
	}
	tree.Entries = make([]gitea.GitEntry, rangeEnd-rangeStart)
	for e := rangeStart; e < rangeEnd; e++ {
		i := e - rangeStart
		tree.Entries[i].Path = entries[e].Name()
		tree.Entries[i].Mode = fmt.Sprintf("%06x", entries[e].Mode())
		tree.Entries[i].Type = string(entries[e].Type)
		tree.Entries[i].Size = entries[e].Size()
		tree.Entries[i].SHA = entries[e].ID.String()

		if entries[e].IsDir() {
			copy(treeURL[copyPos:], entries[e].ID.String())
			tree.Entries[i].URL = string(treeURL[:])
		} else {
			copy(blobURL[copyPos:], entries[e].ID.String())
			tree.Entries[i].URL = string(blobURL[:])
		}
	}
	return tree
}
