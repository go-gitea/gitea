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
	switch action {
	case "ignore":
		err = repo_model.WatchRepoWithOptions(ctx, ctx.Doer, ctx.Repo.Repository, repo_model.WatchOptions{Mode: repo_model.WatchModeDont})
	case "participate":
		err = repo_model.WatchRepoWithOptions(ctx, ctx.Doer, ctx.Repo.Repository, repo_model.WatchOptions{Mode: repo_model.WatchModeNone})
	case "watch":
		err = repo_model.WatchRepoWithOptions(ctx, ctx.Doer, ctx.Repo.Repository, repo_model.WatchOptions{Mode: repo_model.WatchModeNormal, WatchPullRequests: true, WatchIssues: true, WatchReleases: true})
	default:
		return // impossible
	}
	if err != nil {
		handleRepoActionError(ctx, err)
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
		Mode:              repo_model.WatchModeNormal,
		WatchPullRequests: ctx.FormBool("pull_requests"),
		WatchIssues:       ctx.FormBool("issues"),
		WatchReleases:     ctx.FormBool("releases"),
	}
	if err := repo_model.WatchRepoWithOptions(ctx, ctx.Doer, ctx.Repo.Repository, opts); err != nil {
		handleRepoActionError(ctx, err)
		return
	}
	ctx.JSONRedirect("")
}
