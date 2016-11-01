// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/models"
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
		repos[i] = watched.APIFormat(&api.Permission{true, true, true})
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
		ctx.Status(204)
	} else {
		ctx.Status(404)
	}
}

// Watch the repo specified in ctx, as the authenticated user
func Watch(ctx *context.APIContext) {
	err := models.WatchRepo(ctx.User.ID, ctx.Repo.Repository.ID, true)
	if err != nil {
		ctx.Status(500)
		return
	}
	ctx.Status(204)
}

// Unwatch the repo specified in ctx, as the authenticated user
func Unwatch(ctx *context.APIContext) {
	err := models.WatchRepo(ctx.User.ID, ctx.Repo.Repository.ID, false)
	if err != nil {
		ctx.Status(500)
		return
	}
	ctx.Status(204)
}
