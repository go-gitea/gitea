// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
)

const tplWatch templates.TplName = "repo/header/watch"
const tplWatchOptionsBtn templates.TplName = "repo/watch_options_button"

func ActionWatch(ctx *context.Context) {
	err := repo_model.WatchRepo(ctx, ctx.Doer, ctx.Repo.Repository, ctx.PathParam("action") == "watch")
	if err != nil {
		handleActionError(ctx, err)
		return
	}

	if ctx.FormString("watch_mode") == "custom" {
		err = repo_model.WatchRepoOptions(ctx, ctx.Doer, ctx.Repo.Repository, repo_model.WatchOptions{
			PullRequests: ctx.FormBool("pull_requests"),
			Issues:       ctx.FormBool("issues"),
			Releases:     ctx.FormBool("releases"),
		})
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
	watchPullRequests := ctx.FormBool("pull_requests")
	watchIssues := ctx.FormBool("issues")
	watchReleases := ctx.FormBool("releases")

	err := repo_model.WatchRepoOptions(ctx, ctx.Doer, ctx.Repo.Repository, repo_model.WatchOptions{
		PullRequests: watchPullRequests,
		Issues:       watchIssues,
		Releases:     watchReleases,
	})
	if err != nil {
		handleActionError(ctx, err)
		return
	}

	ctx.Data["RepoID"] = ctx.Repo.Repository.ID
	ctx.Data["RepoLink"] = ctx.Repo.RepoLink
	ctx.Data["WatchPullRequests"] = watchPullRequests
	ctx.Data["WatchIssues"] = watchIssues
	ctx.Data["WatchReleases"] = watchReleases

	ctx.HTML(http.StatusOK, tplWatchOptionsBtn)
}
