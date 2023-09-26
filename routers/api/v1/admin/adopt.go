// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/v1/utils"
	repo_service "code.gitea.io/gitea/services/repository"
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
	if listOptions.Page == 0 {
		listOptions.Page = 1
	}
	repoNames, count, err := repo_service.ListUnadoptedRepositories(ctx, ctx.FormString("query"), &listOptions)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.SetTotalCountHeader(int64(count))

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

	ctxUser, err := user_model.GetUserByName(ctx, ownerName)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.NotFound()
			return
		}
		ctx.InternalServerError(err)
		return
	}

	// check not a repo
	has, err := repo_model.IsRepositoryModelExist(ctx, ctxUser, repoName)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	isDir, err := util.IsDir(repo_model.RepoPath(ctxUser.Name, repoName))
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	if has || !isDir {
		ctx.NotFound()
		return
	}
	if _, err := repo_service.AdoptRepository(ctx, ctx.Doer, ctxUser, repo_service.CreateRepoOptions{
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

	ctxUser, err := user_model.GetUserByName(ctx, ownerName)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.NotFound()
			return
		}
		ctx.InternalServerError(err)
		return
	}

	// check not a repo
	has, err := repo_model.IsRepositoryModelExist(ctx, ctxUser, repoName)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	isDir, err := util.IsDir(repo_model.RepoPath(ctxUser.Name, repoName))
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	if has || !isDir {
		ctx.NotFound()
		return
	}

	if err := repo_service.DeleteUnadoptedRepository(ctx, ctx.Doer, ctxUser, repoName); err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
