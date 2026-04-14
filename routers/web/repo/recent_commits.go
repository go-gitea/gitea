// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

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
	weeklyStats, err := repo_service.GetCommitsOverTime(ctx, ctx.Repo.Repository)
	if err != nil {
		if errors.Is(err, repo_service.ErrAwaitGeneration) {
			ctx.Status(http.StatusAccepted)
			return
		}
		ctx.ServerError("GetCommitsOverTime", err)
		return
	}

	data := make(map[int64]*repo_service.WeekData, len(weeklyStats))
	for _, stat := range weeklyStats {
		data[stat.Week] = &repo_service.WeekData{
			Week:    stat.Week,
			Commits: int(stat.Commits),
		}
	}
	ctx.JSON(http.StatusOK, data)
}
