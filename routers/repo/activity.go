// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"time"

	"code.gitea.io/gitea/models"
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
	ctx.Data["DateFrom"] = timeFrom.Format("January 2, 2006")
	ctx.Data["DateUntil"] = timeUntil.Format("January 2, 2006")
	ctx.Data["PeriodText"] = ctx.Tr("repo.activity.period." + ctx.Data["Period"].(string))

	var err error
	if ctx.Data["Activity"], err = models.GetActivityStats(ctx.Repo.Repository, timeFrom,
		ctx.Repo.CanRead(models.UnitTypeReleases),
		ctx.Repo.CanRead(models.UnitTypeIssues),
		ctx.Repo.CanRead(models.UnitTypePullRequests),
		ctx.Repo.CanRead(models.UnitTypeCode)); err != nil {
		ctx.ServerError("GetActivityStats", err)
		return
	}

	ctx.HTML(200, tplActivity)
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
	authors, err := models.GetActivityStatsTopAuthors(ctx.Repo.Repository, timeFrom, 10)
	if err != nil {
		ctx.ServerError("GetActivityStatsTopAuthors", err)
		return
	}

	ctx.JSON(200, authors)
}
