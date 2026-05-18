// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/db"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/optional"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"

	"xorm.io/builder"
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
		ctx.APIError(http.StatusUnprocessableEntity, fmt.Sprintf("Invalid type=%q, must be one of: all, public, private", ctx.FormString("type")))
		return optional.None[bool](), false
	}
}

// listUserRepos - List the repositories owned by the given user.
// canSeePrivate indicates whether the caller is authorized to view private repos.
func listUserRepos(ctx *context.APIContext, u *user_model.User, canSeePrivate bool) {
	isPrivate, ok := parseRepoTypeFilter(ctx)
	if !ok {
		return
	}

	if !canSeePrivate {
		// Caller is not authorized to see private repos (unauthenticated or
		// public-only token scope). If type=private was requested explicitly,
		// short-circuit so that neither the body nor X-Total-Count leaks
		// private repo existence.
		if isPrivate.Has() && isPrivate.Value() {
			ctx.SetLinkHeader(0, utils.GetListOptions(ctx).PageSize)
			ctx.SetTotalCountHeader(0)
			ctx.JSON(http.StatusOK, &[]*api.Repository{})
			return
		}
		// For type=all or type=public, restrict to public only.
		isPrivate = optional.Some(false)
	}

	opts := repo_model.SearchRepoOptions{
		ListOptions: utils.GetListOptions(ctx),
		OrderBy:     db.SearchOrderByID,
	}

	// Build query condition: only repos owned by u, with optional visibility filter.
	var cond builder.Cond = builder.Eq{"owner_id": u.ID}
	if isPrivate.Has() {
		cond = cond.And(builder.Eq{"is_private": isPrivate.Value()})
	}

	// Scope to repositories visible to the requester. For all callers who
	// are not the owner and not an admin (including unauthenticated callers
	// and public-only tokens), apply AccessibleRepositoryCondition so that
	// both the result set and X-Total-Count only include repositories they
	// can actually access.
	if ctx.Doer == nil || (!ctx.Doer.IsAdmin && ctx.Doer.ID != u.ID) {
		cond = cond.And(repo_model.AccessibleRepositoryCondition(ctx.Doer, unit.TypeInvalid))
	}

	repos, count, err := repo_model.SearchRepositoryByCondition(ctx, opts, cond, true)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiRepos := make([]*api.Repository, 0, len(repos))
	for i := range repos {
		permission, err := access_model.GetDoerRepoPermission(ctx, repos[i], ctx.Doer)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		apiRepos = append(apiRepos, convert.ToRepo(ctx, repos[i], permission))
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

	private := ctx.IsSigned && !ctx.PublicOnly
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
		IncludeDescription: true,
	}

	isPrivate, ok := parseRepoTypeFilter(ctx)
	if !ok {
		return
	}

	// Respect public-only token scopes: a public-only token must not
	// be able to list or filter by private repositories.
	if ctx.PublicOnly {
		if isPrivate.Has() && isPrivate.Value() {
			ctx.SetLinkHeader(0, opts.ListOptions.PageSize)
			ctx.SetTotalCountHeader(0)
			ctx.JSON(http.StatusOK, &[]*api.Repository{})
			return
		}
		isPrivate = optional.Some(false)
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
		permission, err := access_model.GetDoerRepoPermission(ctx, repo, ctx.Doer)
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

	listUserRepos(ctx, ctx.Org.Organization.AsUser(), ctx.IsSigned && !ctx.PublicOnly)
}
