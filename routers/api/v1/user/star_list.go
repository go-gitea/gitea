// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"

	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/convert"
)

func listUserStarListsInternal(ctx *context.APIContext, user *user_model.User) {
	starLists, err := repo_model.GetStarListsByUserID(ctx, user.ID, user.IsSameUser(ctx.Doer))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUserStarListsByUserID", err)
		return
	}

	err = starLists.LoadUser(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadUser", err)
		return
	}

	err = starLists.LoadRepositoryCount(ctx, ctx.Doer)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadRepositoryCount", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToStarLists(ctx, starLists, ctx.Doer))
}

// ListUserStarLists list the given user's star lists
func ListUserStarLists(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/starlists user userGetUserStarLists
	// ---
	// summary: List the given user's star lists
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/StarListSlice"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "501":
	//     "$ref": "#/responses/featureDisabled"
	listUserStarListsInternal(ctx, ctx.ContextUser)
}

// ListOwnStarLists list the authenticated user's star lists
func ListOwnStarLists(ctx *context.APIContext) {
	// swagger:operation GET /user/starlists user userGetOwnStarLists
	// ---
	// summary: List the authenticated user's star lists
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/StarListSlice"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "501":
	//     "$ref": "#/responses/featureDisabled"
	listUserStarListsInternal(ctx, ctx.Doer)
}

// GetStarListRepoInfo gets all star lists of the user together with the information, if the given repo is in the list
func GetStarListRepoInfo(ctx *context.APIContext) {
	// swagger:operation GET /user/starlists/repoinfo/{owner}/{repo} user userGetStarListRepoInfo
	// ---
	// summary: Gets all star lists of the user together with the information, if the given repo is in the list
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to star
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to star
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/StarListRepoInfo"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "501":
	//     "$ref": "#/responses/featureDisabled"
	starLists, err := repo_model.GetStarListsByUserID(ctx, ctx.Doer.ID, true)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetStarListsByUserID", err)
		return
	}

	err = starLists.LoadUser(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadUser", err)
		return
	}

	err = starLists.LoadRepositoryCount(ctx, ctx.Doer)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadRepositoryCount", err)
		return
	}

	err = starLists.LoadRepoIDs(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadRepoIDs", err)
		return
	}

	repoInfo := make([]api.StarListRepoInfo, len(starLists))
	for i, list := range starLists {
		repoInfo[i] = api.StarListRepoInfo{StarList: convert.ToStarList(ctx, list, ctx.Doer), Contains: list.ContainsRepoID(ctx.Repo.Repository.ID)}
	}

	ctx.JSON(http.StatusOK, repoInfo)
}

// CreateStarList creates a star list
func CreateStarList(ctx *context.APIContext) {
	// swagger:operation POST /user/starlists user userCreateStarList
	// ---
	// summary: Creates a star list
	// parameters:
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CreateEditStarListOptions"
	// produces:
	// - application/json
	// responses:
	//   "201":
	//     "$ref": "#/responses/StarList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "501":
	//     "$ref": "#/responses/featureDisabled"
	opts := web.GetForm(ctx).(*api.CreateEditStarListOptions)

	starList, err := repo_model.CreateStarList(ctx, ctx.Doer.ID, opts.Name, opts.Description, opts.IsPrivate)
	if err != nil {
		if repo_model.IsErrStarListExists(err) {
			ctx.Error(http.StatusBadRequest, "CreateStarList", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "CreateStarList", err)
		}
		return
	}

	err = starList.LoadUser(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadUser", err)
		return
	}

	ctx.JSON(http.StatusCreated, starList)
}

func getStarListByNameInternal(ctx *context.APIContext) {
	err := ctx.Starlist.LoadUser(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadUser", err)
		return
	}

	err = ctx.Starlist.LoadRepositoryCount(ctx, ctx.Doer)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadRepositoryCount", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToStarList(ctx, ctx.Starlist, ctx.Doer))
}

// GetUserStarListByName get the star list of the given user with the given name
func GetUserStarListByName(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/starlist/{name} user userGetUserStarListByName
	// ---
	// summary: Get the star list of the given user with the given name
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// - name: name
	//   in: path
	//   description: name of the star list
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/StarList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "501":
	//     "$ref": "#/responses/featureDisabled"
	getStarListByNameInternal(ctx)
}

// GetOwnStarListByName get the star list of the authenticated user with the given name
func GetOwnStarListByName(ctx *context.APIContext) {
	// swagger:operation GET /user/starlist/{name} user userGetOwnStarListByName
	// ---
	// summary: Get the star list of the authenticated user with the given name
	// produces:
	// - application/json
	// parameters:
	// - name: name
	//   in: path
	//   description: name of the star list
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/StarList"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "501":
	//     "$ref": "#/responses/featureDisabled"
	getStarListByNameInternal(ctx)
}

// EditStarList edits a star list
func EditStarList(ctx *context.APIContext) {
	// swagger:operation PATCH /user/starlist/{name} user userEditStarList
	// ---
	// summary: Edits a star list
	// parameters:
	// - name: name
	//   in: path
	//   description: name of the star list
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CreateEditStarListOptions"
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/StarList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "501":
	//     "$ref": "#/responses/featureDisabled"
	opts := web.GetForm(ctx).(*api.CreateEditStarListOptions)

	err := ctx.Starlist.EditData(ctx, opts.Name, opts.Description, opts.IsPrivate)
	if err != nil {
		if repo_model.IsErrStarListExists(err) {
			ctx.Error(http.StatusBadRequest, "EditData", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "EditData", err)
		}
		return
	}

	err = ctx.Starlist.LoadUser(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadUser", err)
		return
	}

	err = ctx.Starlist.LoadRepositoryCount(ctx, ctx.Doer)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadRepositoryCount", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToStarList(ctx, ctx.Starlist, ctx.Doer))
}

// DeleteStarList deletes a star list
func DeleteStarList(ctx *context.APIContext) {
	// swagger:operation DELETE /user/starlist/{name} user userDeleteStarList
	// ---
	// summary: Deletes a star list
	// parameters:
	// - name: name
	//   in: path
	//   description: name of the star list
	//   type: string
	//   required: true
	// produces:
	// - application/json
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "501":
	//     "$ref": "#/responses/featureDisabled"
	err := repo_model.DeleteStarListByID(ctx, ctx.Starlist.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadRepositoryCount", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func getStarListReposInternal(ctx *context.APIContext) {
	opts := utils.GetListOptions(ctx)

	repos, count, err := repo_model.SearchRepository(ctx, &repo_model.SearchRepoOptions{Actor: ctx.Doer, StarListID: ctx.Starlist.ID})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SearchRepository", err)
		return
	}

	err = repos.LoadAttributes(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}

	apiRepos := make([]*api.Repository, 0, len(repos))
	for i := range repos {
		permission, err := access_model.GetUserRepoPermission(ctx, repos[i], ctx.Doer)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
			return
		}
		if ctx.IsSigned && ctx.Doer.IsAdmin || permission.UnitAccessMode(unit_model.TypeCode) >= perm.AccessModeRead {
			apiRepos = append(apiRepos, convert.ToRepo(ctx, repos[i], permission))
		}
	}

	ctx.SetLinkHeader(int(count), opts.PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, &apiRepos)
}

// GetUserStarListRepos get the repos of the star list of the given user with the given name
func GetUserStarListRepos(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/starlist/{name}/repos user userGetUserStarListRepos
	// ---
	// summary: Get the repos of the star list of the given user with the given name
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// - name: name
	//   in: path
	//   description: name of the star list
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "501":
	//     "$ref": "#/responses/featureDisabled"
	getStarListReposInternal(ctx)
}

// GetOwnStarListRepos get the repos of the star list of the authenticated user with the given name
func GetOwnStarListRepos(ctx *context.APIContext) {
	// swagger:operation GET /user/starlist/{name}/repos user userGetOwnStarListRepos
	// ---
	// summary: Get the repos of the star list of the authenticated user with the given name
	// produces:
	// - application/json
	// parameters:
	// - name: name
	//   in: path
	//   description: name of the star list
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryList"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "501":
	//     "$ref": "#/responses/featureDisabled"
	getStarListReposInternal(ctx)
}

// AddRepoToStarList adds a Repo to a Star List
func AddRepoToStarList(ctx *context.APIContext) {
	// swagger:operation PUT /user/starlist/{name}/{owner}/{repo} user userAddRepoToStarList
	// ---
	// summary: Adds a Repo to a Star List
	// parameters:
	// - name: name
	//   in: path
	//   description: name of the star list
	//   type: string
	//   required: true
	// - name: owner
	//   in: path
	//   description: owner of the repo to star
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to star
	//   type: string
	//   required: true
	// produces:
	// - application/json
	// responses:
	//   "201":
	//     "$ref": "#/responses/empty"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "501":
	//     "$ref": "#/responses/featureDisabled"
	err := ctx.Starlist.AddRepo(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "AddRepo", err)
		return
	}
	ctx.Status(http.StatusCreated)
}

// RemoveReoFromStarList removes a Repo from a Star List
func RemoveRepoFromStarList(ctx *context.APIContext) {
	// swagger:operation DELETE /user/starlist/{name}/{owner}/{repo} user userRemoveRepoFromStarList
	// ---
	// summary: Removes a Repo from a Star List
	// parameters:
	// - name: name
	//   in: path
	//   description: name of the star list
	//   type: string
	//   required: true
	// - name: owner
	//   in: path
	//   description: owner of the repo to star
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to star
	//   type: string
	//   required: true
	// produces:
	// - application/json
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "501":
	//     "$ref": "#/responses/featureDisabled"
	err := ctx.Starlist.RemoveRepo(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "RemoveRepo", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// GetStarListByID get a star list by id
func GetStarListByID(ctx *context.APIContext) {
	// swagger:operation GET /starlist/{id} user userGetStarListByID
	// ---
	// summary: Get a star list by id
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the star list to get
	//   type: integer
	//   format: int64
	//   required: true
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/StarList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "501":
	//     "$ref": "#/responses/featureDisabled"
	starList, err := repo_model.GetStarListByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if repo_model.IsErrStarListNotFound(err) {
			ctx.NotFound("GetStarListByID", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetStarListByID", err)
		}
		return
	}

	if !starList.HasAccess(ctx.Doer) {
		ctx.NotFound("GetStarListByID", repo_model.ErrStarListNotFound{ID: ctx.ParamsInt64(":id")})
		return
	}

	err = starList.LoadUser(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadUser", err)
		return
	}

	err = starList.LoadRepositoryCount(ctx, ctx.Doer)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadRepositoryCount", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToStarList(ctx, starList, ctx.Doer))
}
