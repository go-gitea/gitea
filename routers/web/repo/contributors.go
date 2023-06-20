package repo

import (
	"net/http"
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	contributors_model "code.gitea.io/gitea/models/contributors"
	"code.gitea.io/gitea/models/unit"
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

	timeUntil := time.Now()
	var timeFrom time.Time

	timeFrom = timeUntil.Add(-time.Hour * 24)
	ctx.Data["DateFrom"] = timeFrom.UTC().Format(time.RFC3339)
	ctx.Data["DateUntil"] = timeUntil.UTC().Format(time.RFC3339)
	ctx.Data["ContributionTypeText"] = ctx.Tr("repo.contributors.contribution_type." + ctx.Data["ContributionType"].(string))

	var err error
	if ctx.Data["Activity"], err = activities_model.GetActivityStats(ctx, ctx.Repo.Repository, timeFrom,
		ctx.Repo.CanRead(unit.TypeReleases),
		ctx.Repo.CanRead(unit.TypeIssues),
		ctx.Repo.CanRead(unit.TypePullRequests),
		ctx.Repo.CanRead(unit.TypeCode)); err != nil {
		ctx.ServerError("GetActivityStats", err)
		return
	}


	if contributor_stats, err := contributors_model.GetContributorStats(ctx, ctx.Repo.Repository); err != nil {
		ctx.ServerError("GetContributorStats", err)
		return
	} else{
		ctx.PageData["repoContributorsCommitStats"] = contributor_stats
		// contributor_stats[""].Weeks.(map[string]interface{})
	}


	ctx.HTML(http.StatusOK, tplContributors)
}
