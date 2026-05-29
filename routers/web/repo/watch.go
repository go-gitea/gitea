// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
)

const (
	tplWatch           templates.TplName = "repo/header/watch"
	tplWatchOptionsBtn templates.TplName = "repo/watch_options_button"
)

func ActionWatch(ctx *context.Context) {
	doWatch := ctx.PathParam("action") == "watch"

	var watchOptions *repo_model.WatchOptions
	if doWatch && ctx.FormString("watch_mode") == "custom" {
		opts := getWatchOptions(ctx)
		if !validateWatchOptions(ctx, opts) {
			return
		}
		watchOptions = &opts
	}

	err := repo_model.WatchRepo(ctx, ctx.Doer, ctx.Repo.Repository, doWatch)
	if err != nil {
		handleActionError(ctx, err)
		return
	}

	if watchOptions != nil {
		err = repo_model.WatchRepoOptions(ctx, ctx.Doer, ctx.Repo.Repository, *watchOptions)
		if err != nil {
			handleActionError(ctx, err)
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

func ActionWatchOptions(ctx *context.Context) {
	opts := getWatchOptions(ctx)
	if !validateWatchOptions(ctx, opts) {
		return
	}

	err := repo_model.WatchRepoOptions(ctx, ctx.Doer, ctx.Repo.Repository, opts)
	if err != nil {
		handleActionError(ctx, err)
		return
	}

	ctx.Data["RepoID"] = ctx.Repo.Repository.ID
	ctx.Data["RepoLink"] = ctx.Repo.RepoLink
	ctx.Data["WatchPullRequests"] = opts.PullRequests
	ctx.Data["WatchIssues"] = opts.Issues
	ctx.Data["WatchReleases"] = opts.Releases

	ctx.HTML(http.StatusOK, tplWatchOptionsBtn)
}

func getWatchOptions(ctx *context.Context) repo_model.WatchOptions {
	return repo_model.WatchOptions{
		PullRequests: ctx.FormBool("pull_requests"),
		Issues:       ctx.FormBool("issues"),
		Releases:     ctx.FormBool("releases"),
	}
}

func validateWatchOptions(ctx *context.Context, opts repo_model.WatchOptions) bool {
	if opts.PullRequests || opts.Issues || opts.Releases {
		return true
	}
	ctx.JSONError(ctx.Tr("repo.watch.options.required"))
	return false
}
