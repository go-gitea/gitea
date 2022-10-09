// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"strings"
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

const (
	tplActivity base.TplName = "repo/activity"
)

// Activity render the page to show repository latest changes
func Activity(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.activity")
	ctx.Data["PageIsActivity"] = true

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
	ctx.Data["DateFrom"] = formatDate(ctx, &timeFrom)
	ctx.Data["DateUntil"] = formatDate(ctx, &timeUntil)
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

	if ctx.PageData["repoActivityTopAuthors"], err = activities_model.GetActivityStatsTopAuthors(ctx, ctx.Repo.Repository, timeFrom, 10); err != nil {
		ctx.ServerError("GetActivityStatsTopAuthors", err)
		return
	}

	ctx.HTML(http.StatusOK, tplActivity)
}

// ActivityAuthors renders JSON with top commit authors for given time period over all branches
func ActivityAuthors(ctx *context.Context) {
	timeUntil := time.Now()
	var timeFrom time.Time

	switch ctx.Params("period") {
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
		timeFrom = timeUntil.Add(-time.Hour * 168)
	}

	var err error
	authors, err := activities_model.GetActivityStatsTopAuthors(ctx, ctx.Repo.Repository, timeFrom, 10)
	if err != nil {
		ctx.ServerError("GetActivityStatsTopAuthors", err)
		return
	}

	ctx.JSON(http.StatusOK, authors)
}

func formatDate(ctx *context.Context, t *time.Time) string {
	r := strings.NewReplacer(
		"January", ctx.Tr("dates.months.january"),
		"February", ctx.Tr("dates.months.february"),
		"March", ctx.Tr("dates.months.march"),
		"April", ctx.Tr("dates.months.april"),
		"May", ctx.Tr("dates.months.may"),
		"June", ctx.Tr("dates.months.june"),
		"July", ctx.Tr("dates.months.july"),
		"August", ctx.Tr("dates.months.august"),
		"September", ctx.Tr("dates.months.september"),
		"October", ctx.Tr("dates.months.october"),
		"November", ctx.Tr("dates.months.november"),
		"December", ctx.Tr("dates.months.december"))
	return r.Replace(t.Format("January 2, 2006"))
}
