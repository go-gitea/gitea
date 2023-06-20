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

	ctx.Data["Period"] = ctx.Params("period")

	timeUntil := time.Now()
	var timeFrom time.Time

	switch ctx.Data["Period"] {
	case "daily":
		timeFrom = timeUntil.Add(-time.Hour * 24)
	case "halfweekly":
		timeFrom = timeUntil.Add(-time.Hour * 72)
	case "weekly":
		timeFrom = timeUntil.Add(-time.Hour * 168)
	case "monthly":
		timeFrom = timeUntil.AddDate(0, -1, 0)
	case "quarterly":
		timeFrom = timeUntil.AddDate(0, -3, 0)
	case "semiyearly":
		timeFrom = timeUntil.AddDate(0, -6, 0)
	case "yearly":
		timeFrom = timeUntil.AddDate(-1, 0, 0)
	default:
		ctx.Data["Period"] = "weekly"
		timeFrom = timeUntil.Add(-time.Hour * 168)
	}
	ctx.Data["DateFrom"] = timeFrom.UTC().Format(time.RFC3339)
	ctx.Data["DateUntil"] = timeUntil.UTC().Format(time.RFC3339)
	ctx.Data["PeriodText"] = ctx.Tr("repo.activity.period." + ctx.Data["Period"].(string))

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
