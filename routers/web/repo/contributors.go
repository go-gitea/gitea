// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	contributors_model "code.gitea.io/gitea/models/contributors"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

const (
	tplContributors base.TplName = "repo/contributors"
)

// Contributors render the page to show repository contributors graph
func Contributors(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.contributors")
	ctx.Data["PageIsContributors"] = true

	ctx.Data["ContributionType"] = ctx.Params("contribution_type")
	if ctx.Data["ContributionType"] == "" {
		ctx.Data["ContributionType"] = "commits"
	}
	ctx.PageData["contributionType"] = ctx.Data["ContributionType"]

	ctx.Data["ContributionTypeText"] = ctx.Tr("repo.contributors.contribution_type." + ctx.Data["ContributionType"].(string))

	ctx.PageData["repoLink"] = ctx.Repo.RepoLink

	ctx.HTML(http.StatusOK, tplContributors)
}

// ContributorsData renders JSON of contributors along with their weekly commit statistics
func ContributorsData(ctx *context.Context) {
	if contributorStats, err := contributors_model.GetContributorStats(ctx, ctx.Repo.Repository, ""); err != nil {
		ctx.ServerError("GetContributorStats", err)
	} else {
		ctx.JSON(http.StatusOK, contributorStats)
	}
}
