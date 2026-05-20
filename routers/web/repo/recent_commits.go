// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	contribution_model "code.gitea.io/gitea/models/repo/contribution"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
	repo_service "code.gitea.io/gitea/services/repository"
)

const (
	tplRecentCommits templates.TplName = "repo/activity"
)

// RecentCommits renders the page to show recent commit frequency on repository
func RecentCommits(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.activity.navbar.recent_commits")

	ctx.Data["PageIsActivity"] = true
	ctx.Data["PageIsRecentCommits"] = true
	ctx.PageData["repoLink"] = ctx.Repo.RepoLink

	ctx.HTML(http.StatusOK, tplRecentCommits)
}

// RecentCommitsData returns JSON of commits over time data.
func RecentCommitsData(ctx *context.Context) {
	data, err := repo_service.GetContributionsOverTime(ctx, ctx.Repo.Repository, nil, nil, contribution_model.RepoStatCommits)
	if err != nil {
		if errors.Is(err, repo_service.ErrAwaitGeneration) {
			ctx.Status(http.StatusAccepted)
			return
		}
		ctx.ServerError("GetContributionsOverTime", err)
		return
	}
	ctx.JSON(http.StatusOK, data)
}
