// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	contribution_model "gitea.dev/models/repo/contribution"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
	repo_service "gitea.dev/services/repository"
)

const (
	tplCodeFrequency templates.TplName = "repo/activity"
)

// CodeFrequency renders the page to show repository code frequency
func CodeFrequency(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.activity.navbar.code_frequency")

	ctx.Data["PageIsActivity"] = true
	ctx.Data["PageIsCodeFrequency"] = true
	ctx.PageData["repoLink"] = ctx.Repo.RepoLink

	ctx.HTML(http.StatusOK, tplCodeFrequency)
}

// CodeFrequencyData returns JSON of code frequency data
func CodeFrequencyData(ctx *context.Context) {
	if weeklyStats, err := repo_service.GetContributionsOverTime(ctx,
		ctx.Repo.Repository, nil, nil,
		contribution_model.RepoStatAdditions, contribution_model.RepoStatDeletions,
	); err != nil {
		if errors.Is(err, repo_service.ErrAwaitGeneration) {
			ctx.Status(http.StatusAccepted)
			return
		}
		ctx.ServerError("GetRepoCodeFrequencyStats", err)
	} else {
		ctx.JSON(http.StatusOK, weeklyStats)
	}
}
