// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

// getWatchedRepos returns the repos that the user with the specified userID is
// watching
func getWatchedRepos(userID int64, private bool) ([]*api.Repository, error) {
	watchedRepos, err := models.GetWatchedRepos(userID, private)
	if err != nil {
		return nil, err
	}

	repos := make([]*api.Repository, len(watchedRepos))
	for i, watched := range watchedRepos {
		access, err := models.AccessLevel(userID, watched)
		if err != nil {
			return nil, err
		}
		repos[i] = watched.APIFormat(access)
	}
	return repos, nil
}

// GetWatchedRepos returns the repos that the user specified in ctx is watching
func GetWatchedRepos(ctx *context.APIContext) {
	// swagger:route GET /users/{username}/subscriptions userListSubscriptions
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: RepositoryList
	//       500: error

	user := GetUserByParams(ctx)
	private := user.ID == ctx.User.ID
	repos, err := getWatchedRepos(user.ID, private)
	if err != nil {
		ctx.Error(500, "getWatchedRepos", err)
	}
	ctx.JSON(200, &repos)
}

// GetMyWatchedRepos returns the repos that the authenticated user is watching
func GetMyWatchedRepos(ctx *context.APIContext) {
	// swagger:route GET /user/subscriptions userCurrentListSubscriptions
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: RepositoryList
	//       500: error

	repos, err := getWatchedRepos(ctx.User.ID, true)
	if err != nil {
		ctx.Error(500, "getWatchedRepos", err)
	}
	ctx.JSON(200, &repos)
}

// IsWatching returns whether the authenticated user is watching the repo
// specified in ctx
func IsWatching(ctx *context.APIContext) {
	// swagger:route GET /repos/{username}/{reponame}/subscription userCurrentCheckSubscription
	//
	//     Responses:
	//       200: WatchInfo
	//       404: notFound

	if models.IsWatching(ctx.User.ID, ctx.Repo.Repository.ID) {
		ctx.JSON(200, api.WatchInfo{
			Subscribed:    true,
			Ignored:       false,
			Reason:        nil,
			CreatedAt:     ctx.Repo.Repository.Created,
			URL:           subscriptionURL(ctx.Repo.Repository),
			RepositoryURL: repositoryURL(ctx.Repo.Repository),
		})
	} else {
		ctx.Status(404)
	}
}

// Watch the repo specified in ctx, as the authenticated user
func Watch(ctx *context.APIContext) {
	// swagger:route PUT /repos/{username}/{reponame}/subscription userCurrentPutSubscription
	//
	//     Responses:
	//       200: WatchInfo
	//       500: error

	err := models.WatchRepo(ctx.User.ID, ctx.Repo.Repository.ID, true)
	if err != nil {
		ctx.Error(500, "WatchRepo", err)
		return
	}
	ctx.JSON(200, api.WatchInfo{
		Subscribed:    true,
		Ignored:       false,
		Reason:        nil,
		CreatedAt:     ctx.Repo.Repository.Created,
		URL:           subscriptionURL(ctx.Repo.Repository),
		RepositoryURL: repositoryURL(ctx.Repo.Repository),
	})

}

// Unwatch the repo specified in ctx, as the authenticated user
func Unwatch(ctx *context.APIContext) {
	// swagger:route DELETE /repos/{username}/{reponame}/subscription userCurrentDeleteSubscription
	//
	//     Responses:
	//       204: empty
	//       500: error

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
