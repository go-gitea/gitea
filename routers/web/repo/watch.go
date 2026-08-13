// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
)

const tplWatch templates.TplName = "repo/header/watch"

func ActionWatch(ctx *context.Context) {
	action := ctx.PathParam("action")
	var err error
	if action == "ignore" {
		err = repo_model.WatchIgnoreRepo(ctx, ctx.Doer, ctx.Repo.Repository)
	} else {
		all := action == "watch" // "participate" is a watch that subscribes to no event on its own
		err = repo_model.WatchRepoWithOptions(ctx, ctx.Doer, ctx.Repo.Repository, repo_model.WatchOptions{PullRequests: all, Issues: all, Releases: all})
	}
	if err != nil {
		handleActionError(ctx, err)
		return
	}

	watch, err := repo_model.GetWatch(ctx, ctx.Doer.ID, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetWatch", err)
		return
	}
	ctx.Data["RepoWatch"] = watch

	ctx.Data["Repository"], err = repo_model.GetRepositoryByName(ctx, ctx.Repo.Repository.OwnerID, ctx.Repo.Repository.Name)
	if err != nil {
		ctx.ServerError("GetRepositoryByName", err)
		return
	}
	ctx.HTML(http.StatusOK, tplWatch)
}

// ActionWatchOptions watches the repository with a custom selection of events
func ActionWatchOptions(ctx *context.Context) {
	opts := repo_model.WatchOptions{ // clearing every event is allowed, it leaves the participating state
		PullRequests: ctx.FormBool(string(repo_model.WatchPullRequests)),
		Issues:       ctx.FormBool(string(repo_model.WatchIssues)),
		Releases:     ctx.FormBool(string(repo_model.WatchReleases)),
	}
	if err := repo_model.WatchRepoWithOptions(ctx, ctx.Doer, ctx.Repo.Repository, opts); err != nil {
		handleActionError(ctx, err)
		return
	}
	ctx.JSONRedirect("")
}
