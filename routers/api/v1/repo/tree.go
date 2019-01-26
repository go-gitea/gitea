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
	// responses:
	//   "200":
	//     "$ref": "#/responses/GitTreeResponse"
	sha := ctx.Params("sha")
	if len(sha) == 0 {
		ctx.Error(400, "sha not provided", nil)
		return
	}
	tree := GetTreeBySHA(ctx, sha)
	if tree != nil {
		ctx.JSON(200, tree)
	} else {
		ctx.Error(400, "sha invalid", nil)
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

	if len(entries) > 1000 {
		tree.Entries = make([]gitea.GitEntry, 1000)
	} else {
		tree.Entries = make([]gitea.GitEntry, len(entries))
	}
	for e := range entries {
		if e > 1000 {
			tree.Truncated = true
			break
		}

		tree.Entries[e].Path = entries[e].Name()
		tree.Entries[e].Mode = fmt.Sprintf("%06x", entries[e].Mode())
		tree.Entries[e].Type = string(entries[e].Type)
		tree.Entries[e].Size = entries[e].Size()
		tree.Entries[e].SHA = entries[e].ID.String()

		if entries[e].IsDir() {
			copy(treeURL[copyPos:], entries[e].ID.String())
			tree.Entries[e].URL = string(treeURL[:])
		} else {
			copy(blobURL[copyPos:], entries[e].ID.String())
			tree.Entries[e].URL = string(blobURL[:])
		}
	}
	return tree
}
