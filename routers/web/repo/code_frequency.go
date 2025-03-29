// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
	contributors_service "code.gitea.io/gitea/services/repository"
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
	ctx.Data["CanReadCode"] = ctx.Repo.CanRead(unit.TypeCode)

	ctx.HTML(http.StatusOK, tplCodeFrequency)
}

// ContributorStats returns JSON of code contributor stats data
func ContributorStats(ctx *context.Context) {
	if contributorStats, err := contributors_service.GetContributorStats(ctx, ctx.Cache, ctx.Repo.Repository, ctx.Repo.Repository.DefaultBranch); err != nil {
		if errors.Is(err, contributors_service.ErrAwaitGeneration) {
			ctx.Status(http.StatusAccepted)
			return
		}
		ctx.ServerError("ContributorStats", err)
	} else {
		ctx.JSON(http.StatusOK, contributorStats["total"].Weeks)
	}
}
