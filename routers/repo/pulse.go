// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	//"fmt"
	"time"
	//"strings"

	//"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	//"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	//"code.gitea.io/gitea/modules/log"
	//"code.gitea.io/gitea/modules/setting"
)

const (
	tplPulse base.TplName = "repo/pulse"
)

// Pulse render the page to show repository latest changes
func Pulse(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.pulse")
	ctx.Data["PageIsPulse"] = true

	ctx.Data["Period"] = ctx.Params("period")

	timeTill := time.Now()
	var timeFrom time.Time

	switch ctx.Data["Period"] {
	case "daily":
		timeFrom = timeTill.Add(-time.Hour * 24)
	case "halfweekly":
		timeFrom = timeTill.Add(-time.Hour * 72)
	case "weekly":
		timeFrom = timeTill.Add(-time.Hour * 168)
	case "monthly":
		timeFrom = timeTill.AddDate(0, -1, 0)
	default:
		ctx.Data["Period"] = "weekly"
		timeFrom = timeTill.Add(-time.Hour * 168)
	}
	ctx.Data["DateFrom"] = timeFrom.Format("January 2, 2006")
	ctx.Data["DateTill"] = timeTill.Format("January 2, 2006")
	ctx.Data["PeriodText"] = ctx.Tr("repo.pulse.period." + ctx.Data["Period"].(string))

	stats := &models.PulseStats{}

	if err := models.FillPullRequestsForPulse(stats, ctx.Repo.Repository.ID, timeFrom); err != nil {
		ctx.Error(500, "FillPullRequestsForPulse: "+err.Error())
		return
	}
	if err := models.FillIssuesForPulse(stats, ctx.Repo.Repository.ID, timeFrom); err != nil {
		ctx.Error(500, "FillIssuesForPulse: "+err.Error())
		return
	}

	ctx.Data["Pulse"] = stats

	ctx.HTML(200, tplPulse)
}
