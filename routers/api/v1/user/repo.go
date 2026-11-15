// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"

	"gitea.dev/models/db"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

func repoListOrder(ctx *context.APIContext, defaultOrder db.SearchOrderBy) (db.SearchOrderBy, bool) {
	orderBy, ok := utils.ResolveSortOrder(ctx, repo_model.OrderByMap, defaultOrder)
	if !ok || orderBy == "" || orderBy == db.SearchOrderByID || orderBy == db.SearchOrderByIDReverse {
		return orderBy, ok
	}
	tiebreaker := db.SearchOrderByID
	if ctx.FormString("order") == "desc" {
		tiebreaker = db.SearchOrderByIDReverse
	}
	return orderBy + ", " + tiebreaker, true
}

// listUserRepos - List the repositories owned by the given user.
func listUserRepos(ctx *context.APIContext, u *user_model.User, private bool) {
	opts := utils.GetListOptions(ctx)
	orderBy, ok := repoListOrder(ctx, db.SearchOrderByID)
	if !ok {
		return
	}

	repos, count, err := repo_model.GetUserRepositories(ctx, repo_model.SearchRepoOptions{
		OwnerID:     u.ID,
		Actor:       ctx.Doer,
		Private:     private,
		ListOptions: opts,
		OrderBy:     orderBy,
	})
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
		permission, err := access_model.GetDoerRepoPermission(ctx, repos[i], ctx.Doer)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		if ctx.IsSigned && ctx.Doer.IsAdmin || permission.HasAnyUnitAccess() {
			apiRepos = append(apiRepos, convert.ToRepo(ctx, repos[i], permission))
		}
	}

	ctx.SetLinkHeader(count, opts.PageSize)
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
	// - name: sort
	//   in: query
	//   description: sort repos by attribute. Supported values are
	//                "alpha", "created", "updated", "size", "git_size", "lfs_size", "stars", "forks" and "id".
	//                Default is "id".
	//   type: string
	// - name: order
	//   in: query
	//   description: sort order, either "asc" (ascending) or "desc" (descending).
	//                Default is "asc", ignored if "sort" is not specified.
	//   type: string
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
	//   default:
	//     "$ref": "#/responses/error"

	listUserRepos(ctx, ctx.ContextUser, ctx.IsSigned)
}

// ListMyRepos - list the repositories you own or have access to.
func ListMyRepos(ctx *context.APIContext) {
	// swagger:operation GET /user/repos user userCurrentListRepos
	// ---
	// summary: List the repos that the authenticated user owns
	// produces:
	// - application/json
	// parameters:
	// - name: sort
	//   in: query
	//   description: sort repos by attribute. Supported values are
	//                "alpha", "created", "updated", "size", "git_size", "lfs_size", "stars", "forks" and "id".
	//                Default is "alpha".
	//   type: string
	// - name: order
	//   in: query
	//   description: sort order, either "asc" (ascending) or "desc" (descending).
	//                Default is "asc", ignored if "sort" is not specified.
	//   type: string
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

	orderBy, ok := repoListOrder(ctx, "")
	if !ok {
		return
	}
	opts := repo_model.SearchRepoOptions{
		OrderBy:            orderBy,
		ListOptions:        utils.GetListOptions(ctx),
		Actor:              ctx.Doer,
		OwnerID:            ctx.Doer.ID,
		Private:            ctx.IsSigned,
		IncludeDescription: true,
	}

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
			return
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
	// - name: sort
	//   in: query
	//   description: sort repos by attribute. Supported values are
	//                "alpha", "created", "updated", "size", "git_size", "lfs_size", "stars", "forks" and "id".
	//                Default is "id".
	//   type: string
	// - name: order
	//   in: query
	//   description: sort order, either "asc" (ascending) or "desc" (descending).
	//                Default is "asc", ignored if "sort" is not specified.
	//   type: string
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
	//   default:
	//     "$ref": "#/responses/error"

	listUserRepos(ctx, ctx.Org.Organization.AsUser(), ctx.IsSigned)
}
