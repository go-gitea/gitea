// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"time"

	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

// WatchInfo contains information about a watched repository
type WatchInfo struct {
	Subscribed    bool        `json:"subscribed"`
	Ignored       bool        `json:"ignored"`
	Reason        interface{} `json:"reason"`
	CreatedAt     time.Time   `json:"created_at"`
	URL           string      `json:"url"`
	RepositoryURL string      `json:"repository_url"`
}

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
	repos, err := getWatchedRepos(ctx.User.ID, true)
	if err != nil {
		ctx.Error(500, "getWatchedRepos", err)
	}
	ctx.JSON(200, &repos)
}

// IsWatching returns whether the authenticated user is watching the repo
// specified in ctx
func IsWatching(ctx *context.APIContext) {
	if models.IsWatching(ctx.User.ID, ctx.Repo.Repository.ID) {
		ctx.JSON(200, WatchInfo{
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
	err := models.WatchRepo(ctx.User.ID, ctx.Repo.Repository.ID, true)
	if err != nil {
		ctx.Error(500, "WatchRepo", err)
		return
	}
	ctx.JSON(200, WatchInfo{
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
