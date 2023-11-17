// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	std_context "context"
	"net/http"

	"code.gitea.io/gitea/models/db"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/convert"
)

// getWatchedRepos returns the repos that the user with the specified userID is watching
func getWatchedRepos(ctx std_context.Context, user *user_model.User, private bool, listOptions db.ListOptions) ([]*api.Repository, int64, error) {
	watchedRepos, total, err := repo_model.GetWatchedRepos(ctx, user.ID, private, listOptions)
	if err != nil {
		return nil, 0, err
	}

	repos := make([]*api.Repository, len(watchedRepos))
	for i, watched := range watchedRepos {
		permission, err := access_model.GetUserRepoPermission(ctx, watched, user)
		if err != nil {
			return nil, 0, err
		}
		repos[i] = convert.ToRepo(ctx, watched, permission)
	}
	return repos, total, nil
}

// GetWatchedRepos returns the repos that the user specified in ctx is watching
func GetWatchedRepos(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/subscriptions user userListSubscriptions
	// ---
	// summary: List the repositories watched by a user
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   type: string
	//   in: path
	//   description: username of the user
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

	private := ctx.ContextUser.ID == ctx.Doer.ID
	repos, total, err := getWatchedRepos(ctx, ctx.ContextUser, private, utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "getWatchedRepos", err)
	}

	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, &repos)
}

// GetMyWatchedRepos returns the repos that the authenticated user is watching
func GetMyWatchedRepos(ctx *context.APIContext) {
	// swagger:operation GET /user/subscriptions user userCurrentListSubscriptions
	// ---
	// summary: List repositories watched by the authenticated user
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryList"

	repos, total, err := getWatchedRepos(ctx, ctx.Doer, true, utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "getWatchedRepos", err)
	}

	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, &repos)
}

// IsWatching returns whether the authenticated user is watching the repo
// specified in ctx
func IsWatching(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/subscription repository userCurrentCheckSubscription
	// ---
	// summary: Check if the current user is watching a repo
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
	//   "200":
	//     "$ref": "#/responses/WatchInfo"
	//   "404":
	//     description: User is not watching this repo or repo do not exist

	if repo_model.IsWatching(ctx, ctx.Doer.ID, ctx.Repo.Repository.ID) {
		ctx.JSON(http.StatusOK, api.WatchInfo{
			Subscribed:    true,
			Ignored:       false,
			Reason:        nil,
			CreatedAt:     ctx.Repo.Repository.CreatedUnix.AsTime(),
			URL:           subscriptionURL(ctx.Repo.Repository),
			RepositoryURL: ctx.Repo.Repository.APIURL(),
		})
	} else {
		ctx.NotFound()
	}
}

// Watch the repo specified in ctx, as the authenticated user
func Watch(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/subscription repository userCurrentPutSubscription
	// ---
	// summary: Watch a repo
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
	//   "200":
	//     "$ref": "#/responses/WatchInfo"
	//   "404":
	//     "$ref": "#/responses/notFound"

	err := repo_model.WatchRepo(ctx, ctx.Doer.ID, ctx.Repo.Repository.ID, true)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "WatchRepo", err)
		return
	}
	ctx.JSON(http.StatusOK, api.WatchInfo{
		Subscribed:    true,
		Ignored:       false,
		Reason:        nil,
		CreatedAt:     ctx.Repo.Repository.CreatedUnix.AsTime(),
		URL:           subscriptionURL(ctx.Repo.Repository),
		RepositoryURL: ctx.Repo.Repository.APIURL(),
	})
}

// Unwatch the repo specified in ctx, as the authenticated user
func Unwatch(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/subscription repository userCurrentDeleteSubscription
	// ---
	// summary: Unwatch a repo
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

	err := repo_model.WatchRepo(ctx, ctx.Doer.ID, ctx.Repo.Repository.ID, false)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "UnwatchRepo", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// subscriptionURL returns the URL of the subscription API endpoint of a repo
func subscriptionURL(repo *repo_model.Repository) string {
	return repo.APIURL() + "/subscription"
}
