package repo

import (
	"net/http"
	"time"

	contributors_model "code.gitea.io/gitea/models/contributors"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

const (
	tplContributors base.TplName = "repo/contributors"
)

// Contributors render the page to show repository contributors graph
func Contributors(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.activity")
	ctx.Data["PageIsContributors"] = true

	ctx.Data["ContributionType"] = ctx.Params("contribution_type")
	if ctx.Data["ContributionType"] == "" {
		ctx.Data["ContributionType"] = "commits"
	}
	ctx.PageData["contributionType"] = ctx.Data["ContributionType"]

	ctx.Data["ContributionTypeText"] = ctx.Tr("repo.contributors.contribution_type." + ctx.Data["ContributionType"].(string))

	if contributor_stats, err := contributors_model.GetContributorStats(ctx, ctx.Repo.Repository); err != nil {
		ctx.ServerError("GetContributorStats", err)
		return
	} else {
		ctx.PageData["repoContributorsCommitStats"] = contributor_stats
		timeUntil := time.Now()
		timeFrom := time.UnixMilli(contributor_stats[""].Weeks[0].Week)

		ctx.Data["DateFrom"] = timeFrom.UTC().Format(time.RFC3339)
		ctx.Data["DateUntil"] = timeUntil.UTC().Format(time.RFC3339)
		// contributor_stats[""].Weeks.(map[string]interface{})
	}

	ctx.HTML(http.StatusOK, tplContributors)
}
