// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package user

import (
	"errors"
	"net/http"

	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// getStarredRepos returns the repos that the user with the specified userID has
// starred
func getStarredRepos(ctx *context.APIContext, user *user_model.User, private bool) ([]*api.Repository, error) {
	starredRepos, err := repo_model.GetStarredRepos(ctx, &repo_model.StarredReposOptions{
		ListOptions:    utils.GetListOptions(ctx),
		StarrerID:      user.ID,
		IncludePrivate: private,
	})
	if err != nil {
		return nil, err
	}

	repos := make([]*api.Repository, len(starredRepos))
	for i, starred := range starredRepos {
		permission, err := access_model.GetUserRepoPermission(ctx, starred, user)
		if err != nil {
			return nil, err
		}
		repos[i] = convert.ToRepo(ctx, starred, permission)
	}
	return repos, nil
}

// GetStarredRepos returns the repos that the given user has starred
func GetStarredRepos(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/starred user userListStarred
	// ---
	// summary: The repos that the given user has starred
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
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
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	private := ctx.ContextUser.ID == ctx.Doer.ID
	repos, err := getStarredRepos(ctx, ctx.ContextUser, private)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetTotalCountHeader(int64(ctx.ContextUser.NumStars))
	ctx.JSON(http.StatusOK, &repos)
}

// GetMyStarredRepos returns the repos that the authenticated user has starred
func GetMyStarredRepos(ctx *context.APIContext) {
	// swagger:operation GET /user/starred user userCurrentListStarred
	// ---
	// summary: The repos that the authenticated user has starred
	// parameters:
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryList"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	repos, err := getStarredRepos(ctx, ctx.Doer, true)
	if err != nil {
		ctx.APIErrorInternal(err)
	}

	ctx.SetTotalCountHeader(int64(ctx.Doer.NumStars))
	ctx.JSON(http.StatusOK, &repos)
}

// IsStarring returns whether the authenticated is starring the repo
func IsStarring(ctx *context.APIContext) {
	// swagger:operation GET /user/starred/{owner}/{repo} user userCurrentCheckStarring
	// ---
	// summary: Whether the authenticated is starring the repo
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

	if repo_model.IsStaring(ctx, ctx.Doer.ID, ctx.Repo.Repository.ID) {
		ctx.Status(http.StatusNoContent)
	} else {
		ctx.APIErrorNotFound()
	}
}

// Star the repo specified in the APIContext, as the authenticated user
func Star(ctx *context.APIContext) {
	// swagger:operation PUT /user/starred/{owner}/{repo} user userCurrentPutStar
	// ---
	// summary: Star the given repo
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
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	err := repo_model.StarRepo(ctx, ctx.Doer, ctx.Repo.Repository, true)
	if err != nil {
		if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.APIError(http.StatusForbidden, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}

// Unstar the repo specified in the APIContext, as the authenticated user
func Unstar(ctx *context.APIContext) {
	// swagger:operation DELETE /user/starred/{owner}/{repo} user userCurrentDeleteStar
	// ---
	// summary: Unstar the given repo
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to unstar
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to unstar
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	err := repo_model.StarRepo(ctx, ctx.Doer, ctx.Repo.Repository, false)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
