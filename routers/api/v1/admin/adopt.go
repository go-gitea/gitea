// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/v1/utils"
)

// ListUnadoptedRepositories lists the unadopted repositories that match the provided names
func ListUnadoptedRepositories(ctx *context.APIContext) {
	// swagger:operation GET /admin/unadopted admin adminUnadoptedList
	// ---
	// summary: List unadopted repositories
	// produces:
	// - application/json
	// parameters:
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// - name: pattern
	//   in: query
	//   description: pattern of repositories to search for
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/StringSlice"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	listOptions := utils.GetListOptions(ctx)
	repoNames, count, err := repository.ListUnadoptedRepositories(ctx.FormString("query"), &listOptions)
	if err != nil {
		ctx.InternalServerError(err)
	}

	ctx.Header().Set("X-Total-Count", fmt.Sprintf("%d", count))
	ctx.Header().Set("Access-Control-Expose-Headers", "X-Total-Count")

	ctx.JSON(http.StatusOK, repoNames)
}

// AdoptRepository will adopt an unadopted repository
func AdoptRepository(ctx *context.APIContext) {
	// swagger:operation POST /admin/unadopted/{owner}/{repo} admin adminAdoptRepository
	// ---
	// summary: Adopt unadopted files as a repository
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
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	ownerName := ctx.Params(":username")
	repoName := ctx.Params(":reponame")

	ctxUser, err := models.GetUserByName(ownerName)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.NotFound()
			return
		}
		ctx.InternalServerError(err)
		return
	}

	// check not a repo
	has, err := models.IsRepositoryExist(ctxUser, repoName)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	isDir, err := util.IsDir(models.RepoPath(ctxUser.Name, repoName))
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	if has || !isDir {
		ctx.NotFound()
		return
	}
	if _, err := repository.AdoptRepository(ctx.User, ctxUser, models.CreateRepoOptions{
		Name:      repoName,
		IsPrivate: true,
	}); err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// DeleteUnadoptedRepository will delete an unadopted repository
func DeleteUnadoptedRepository(ctx *context.APIContext) {
	// swagger:operation DELETE /admin/unadopted/{owner}/{repo} admin adminDeleteUnadoptedRepository
	// ---
	// summary: Delete unadopted files
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
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	ownerName := ctx.Params(":username")
	repoName := ctx.Params(":reponame")

	ctxUser, err := models.GetUserByName(ownerName)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.NotFound()
			return
		}
		ctx.InternalServerError(err)
		return
	}

	// check not a repo
	has, err := models.IsRepositoryExist(ctxUser, repoName)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	isDir, err := util.IsDir(models.RepoPath(ctxUser.Name, repoName))
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	if has || !isDir {
		ctx.NotFound()
		return
	}

	if err := repository.DeleteUnadoptedRepository(ctx.User, ctxUser, repoName); err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
