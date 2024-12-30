// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/services/context"
	files_service "code.gitea.io/gitea/services/repository/files"
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
	//   description: number of items per page
	//   required: false
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/GitTreeResponse"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	sha := ctx.PathParam("sha")
	if len(sha) == 0 {
		ctx.Error(http.StatusBadRequest, "", "sha not provided")
		return
	}
	if tree, err := files_service.GetTreeBySHA(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, sha, ctx.FormInt("page"), ctx.FormInt("per_page"), ctx.FormBool("recursive")); err != nil {
		ctx.Error(http.StatusBadRequest, "", err.Error())
	} else {
		ctx.SetTotalCountHeader(int64(tree.TotalCount))
		ctx.JSON(http.StatusOK, tree)
	}
}
