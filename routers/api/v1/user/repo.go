// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"

	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/optional"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// parseRepoTypeFilter parses the "type" query parameter into an IsPrivate filter.
// Returns (filter, true) on success, or (zero, false) after writing a 422 response on invalid input.
func parseRepoTypeFilter(ctx *context.APIContext) (optional.Option[bool], bool) {
	switch ctx.FormString("type") {
	case "", "all":
		return optional.None[bool](), true
	case "public":
		return optional.Some(false), true
	case "private":
		return optional.Some(true), true
	default:
		ctx.APIError(http.StatusUnprocessableEntity, "Invalid type, must be one of: all, public, private")
		return optional.None[bool](), false
	}
}

// listUserRepos - List the repositories owned by the given user.
func listUserRepos(ctx *context.APIContext, u *user_model.User, private bool) {
	isPrivate, ok := parseRepoTypeFilter(ctx)
	if !ok {
		return
	}

	if !private {
		// Unauthenticated caller: can only see public repos.
		// If type=private was requested explicitly, short-circuit so that
		// neither the body nor X-Total-Count leaks private repo existence.
		if isPrivate.Has() && isPrivate.Value() {
			ctx.SetTotalCountHeader(0)
			ctx.JSON(http.StatusOK, &[]*api.Repository{})
			return
		}
		// For type=all or type=public, restrict to public only.
		isPrivate = optional.Some(false)
	}

	opts := repo_model.SearchRepoOptions{
		Actor:       u,
		IsPrivate:   isPrivate,
		ListOptions: utils.GetListOptions(ctx),
		OrderBy:     "id ASC",
	}

	repos, count, err := repo_model.GetUserRepositories(ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if err := repos.LoadAttributes(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiRepos := make([]*api.Repository, 0, len(repos))
	for i := range repos {
		permission, err := access_model.GetUserRepoPermission(ctx, repos[i], ctx.Doer)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		if ctx.IsSigned && ctx.Doer.IsAdmin || permission.HasAnyUnitAccess() {
			apiRepos = append(apiRepos, convert.ToRepo(ctx, repos[i], permission))
		}
	}

	ctx.SetLinkHeader(count, opts.ListOptions.PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, &apiRepos)
}

// ListUserRepos - list the repos owned by the given user.
func ListUserRepos(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/repos user userListRepos
	// ---
	// summary: List the repos owned by the given user
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user whose owned repos are to be listed
	//   type: string
	//   required: true
	// - name: type
	//   in: query
	//   description: filter by type, "all" (default), "public", or "private"
	//   type: string
	//   enum: [all, public, private]
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
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	private := ctx.IsSigned
	listUserRepos(ctx, ctx.ContextUser, private)
}

// ListMyRepos - list the repositories you own or have access to.
func ListMyRepos(ctx *context.APIContext) {
	// swagger:operation GET /user/repos user userCurrentListRepos
	// ---
	// summary: List the repos that the authenticated user owns
	// produces:
	// - application/json
	// parameters:
	// - name: type
	//   in: query
	//   description: filter by type, "all" (default), "public", or "private"
	//   type: string
	//   enum: [all, public, private]
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
	//   "422":
	//     "$ref": "#/responses/validationError"

	opts := repo_model.SearchRepoOptions{
		ListOptions:        utils.GetListOptions(ctx),
		Actor:              ctx.Doer,
		OwnerID:            ctx.Doer.ID,
		Private:            ctx.IsSigned,
		IncludeDescription: true,
	}

	isPrivate, ok := parseRepoTypeFilter(ctx)
	if !ok {
		return
	}
	opts.IsPrivate = isPrivate

	repos, count, err := repo_model.SearchRepository(ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	results := make([]*api.Repository, len(repos))
	for i, repo := range repos {
		if err = repo.LoadOwner(ctx); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		permission, err := access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
		if err != nil {
			ctx.APIErrorInternal(err)
		}
		results[i] = convert.ToRepo(ctx, repo, permission)
	}

	ctx.SetLinkHeader(count, opts.ListOptions.PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, &results)
}

// ListOrgRepos - list the repositories of an organization.
func ListOrgRepos(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/repos organization orgListRepos
	// ---
	// summary: List an organization's repos
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: type
	//   in: query
	//   description: filter by type, "all" (default), "public", or "private"
	//   type: string
	//   enum: [all, public, private]
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
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	listUserRepos(ctx, ctx.Org.Organization.AsUser(), ctx.IsSigned)
}
