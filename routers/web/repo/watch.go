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
		err = repo_model.IgnoreRepo(ctx, ctx.Doer, ctx.Repo.Repository)
	} else {
		err = repo_model.WatchRepo(ctx, ctx.Doer, ctx.Repo.Repository, action == "watch")
	}
	if err != nil {
		handleActionError(ctx, err)
		return
	}
	if action == "watch" { // watching again always restores every event, so "all activity" can undo a custom selection
		opts := repo_model.WatchOptions{PullRequests: true, Issues: true, Releases: true}
		if err := repo_model.SetWatchOptions(ctx, ctx.Doer.ID, ctx.Repo.Repository.ID, opts); err != nil {
			ctx.ServerError("SetWatchOptions", err)
			return
		}
	}

	watch, err := repo_model.GetWatch(ctx, ctx.Doer.ID, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetWatch", err)
		return
	}
	ctx.Data["Watch"] = watch
	ctx.Data["IsWatchingRepo"] = repo_model.IsWatchMode(watch.Mode)

	ctx.Data["Repository"], err = repo_model.GetRepositoryByName(ctx, ctx.Repo.Repository.OwnerID, ctx.Repo.Repository.Name)
	if err != nil {
		ctx.ServerError("GetRepositoryByName", err)
		return
	}
	ctx.HTML(http.StatusOK, tplWatch)
}

// ActionWatchOptions watches the repository with a custom selection of events
func ActionWatchOptions(ctx *context.Context) {
	opts := repo_model.WatchOptions{
		PullRequests: ctx.FormBool(string(repo_model.WatchPullRequests)),
		Issues:       ctx.FormBool(string(repo_model.WatchIssues)),
		Releases:     ctx.FormBool(string(repo_model.WatchReleases)),
	}
	if !opts.PullRequests && !opts.Issues && !opts.Releases {
		ctx.JSONError(ctx.Tr("repo.watch.options.required"))
		return
	}
	if err := repo_model.WatchRepo(ctx, ctx.Doer, ctx.Repo.Repository, true); err != nil {
		handleActionError(ctx, err)
		return
	}
	if err := repo_model.SetWatchOptions(ctx, ctx.Doer.ID, ctx.Repo.Repository.ID, opts); err != nil {
		ctx.ServerError("SetWatchOptions", err)
		return
	}
	ctx.JSONRedirect("")
}
