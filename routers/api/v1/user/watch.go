// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	api "code.gitea.io/gitea/modules/structs"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

// getWatchedRepos returns the repos that the user with the specified userID is
// watching
func getWatchedRepos(user *models.User, private bool) ([]*api.Repository, error) {
	watchedRepos, err := models.GetWatchedRepos(user.ID, private)
	if err != nil {
		return nil, err
	}

	repos := make([]*api.Repository, len(watchedRepos))
	for i, watched := range watchedRepos {
		access, err := models.AccessLevel(user, watched)
		if err != nil {
			return nil, err
		}
		repos[i] = watched.APIFormat(access)
	}
	return repos, nil
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryList"
	user := GetUserByParams(ctx)
	private := user.ID == ctx.User.ID
	repos, err := getWatchedRepos(user, private)
	if err != nil {
		ctx.Error(500, "getWatchedRepos", err)
	}
	ctx.JSON(200, &repos)
}

// GetMyWatchedRepos returns the repos that the authenticated user is watching
func GetMyWatchedRepos(ctx *context.APIContext) {
	// swagger:operation GET /user/subscriptions user userCurrentListSubscriptions
	// ---
	// summary: List repositories watched by the authenticated user
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryList"
	repos, err := getWatchedRepos(ctx.User, true)
	if err != nil {
		ctx.Error(500, "getWatchedRepos", err)
	}
	ctx.JSON(200, &repos)
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
	if models.IsWatching(ctx.User.ID, ctx.Repo.Repository.ID) {
		ctx.JSON(200, api.WatchInfo{
			Subscribed:    true,
			Ignored:       false,
			Reason:        nil,
			CreatedAt:     ctx.Repo.Repository.CreatedUnix.AsTime(),
			URL:           subscriptionURL(ctx.Repo.Repository),
			RepositoryURL: repositoryURL(ctx.Repo.Repository),
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
	err := models.WatchRepo(ctx.User.ID, ctx.Repo.Repository.ID, true)
	if err != nil {
		ctx.Error(500, "WatchRepo", err)
		return
	}
	ctx.JSON(200, api.WatchInfo{
		Subscribed:    true,
		Ignored:       false,
		Reason:        nil,
		CreatedAt:     ctx.Repo.Repository.CreatedUnix.AsTime(),
		URL:           subscriptionURL(ctx.Repo.Repository),
		RepositoryURL: repositoryURL(ctx.Repo.Repository),
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
	err := models.WatchRepo(ctx.User.ID, ctx.Repo.Repository.ID, false)
	if err != nil {
		ctx.Error(500, "UnwatchRepo", err)
		return
	}
	ctx.Status(204)
}

// subscriptionURL returns the URL of the subscription API endpoint of a repo
func subscriptionURL(repo *models.Repository) string {
	return repositoryURL(repo) + "/subscription"
}

// repositoryURL returns the URL of the API endpoint of a repo
func repositoryURL(repo *models.Repository) string {
	return setting.AppURL + "api/v1/" + repo.FullName()
}
