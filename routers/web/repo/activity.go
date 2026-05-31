// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"time"

	activities_model "gitea.dev/models/activities"
	git_model "gitea.dev/models/git"
	"gitea.dev/models/unit"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
)

const tplActivity templates.TplName = "repo/activity"

const activityDateLayout = "2006-01-02"

func activityDateRange(period, from, until string, now time.Time) (string, time.Time, time.Time, bool) {
	timeUntil := now
	timeFrom := timeUntil.Add(-time.Hour * 168)
	selectedPeriod := "weekly"
	switch period {
	case "daily":
		selectedPeriod, timeFrom = "daily", timeUntil.Add(-time.Hour*24)
	case "halfweekly":
		selectedPeriod, timeFrom = "halfweekly", timeUntil.Add(-time.Hour*72)
	case "weekly":
		selectedPeriod, timeFrom = "weekly", timeUntil.Add(-time.Hour*168)
	case "monthly":
		selectedPeriod, timeFrom = "monthly", timeUntil.AddDate(0, -1, 0)
	case "quarterly":
		selectedPeriod, timeFrom = "quarterly", timeUntil.AddDate(0, -3, 0)
	case "semiyearly":
		selectedPeriod, timeFrom = "semiyearly", timeUntil.AddDate(0, -6, 0)
	case "yearly":
		selectedPeriod, timeFrom = "yearly", timeUntil.AddDate(-1, 0, 0)
	}

	if from == "" || until == "" {
		return selectedPeriod, timeFrom, timeUntil, false
	}

	customFrom, errFrom := time.ParseInLocation(activityDateLayout, from, time.Local)
	customUntil, errUntil := time.ParseInLocation(activityDateLayout, until, time.Local)
	if errFrom != nil || errUntil != nil {
		return selectedPeriod, timeFrom, timeUntil, false
	}
	customUntil = customUntil.AddDate(0, 0, 1).Add(-time.Second)
	if customFrom.After(customUntil) {
		return selectedPeriod, timeFrom, timeUntil, false
	}
	return "custom", customFrom, customUntil, true
}

// Activity render the page to show repository latest changes
func Activity(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.activity")
	ctx.Data["PageIsActivity"] = true

	ctx.Data["PageIsPulse"] = true

	period, timeFrom, timeUntil, customPeriod := activityDateRange(ctx.PathParam("period"), ctx.FormString("from"), ctx.FormString("until"), time.Now())
	ctx.Data["DateFrom"] = timeFrom
	ctx.Data["DateUntil"] = timeUntil
	ctx.Data["DateFromValue"] = timeFrom.Format(activityDateLayout)
	ctx.Data["DateUntilValue"] = timeUntil.Format(activityDateLayout)
	ctx.Data["Period"] = period
	if customPeriod {
		ctx.Data["PeriodText"] = ctx.Tr("repo.activity.period.custom")
	} else {
		ctx.Data["PeriodText"] = ctx.Tr("repo.activity.period." + period)
	}

	canReadCode := ctx.Repo.Permission.CanRead(unit.TypeCode)
	if canReadCode {
		// GetActivityStats needs to read the default branch to get some information
		branchExist, _ := git_model.IsBranchExist(ctx, ctx.Repo.Repository.ID, ctx.Repo.Repository.DefaultBranch)
		if !branchExist {
			ctx.Data["NotFoundPrompt"] = ctx.Tr("repo.branch.default_branch_not_exist", ctx.Repo.Repository.DefaultBranch)
			ctx.NotFound(nil)
			return
		}
	}

	var err error
	// TODO: refactor these arguments to a struct
	ctx.Data["Activity"], err = activities_model.GetActivityStats(ctx, ctx.Repo.Repository, timeFrom, timeUntil,
		ctx.Repo.Permission.CanRead(unit.TypeReleases),
		ctx.Repo.Permission.CanRead(unit.TypeIssues),
		ctx.Repo.Permission.CanRead(unit.TypePullRequests),
		canReadCode,
	)
	if err != nil {
		ctx.ServerError("GetActivityStats", err)
		return
	}

	if ctx.PageData["repoActivityTopAuthors"], err = activities_model.GetActivityStatsTopAuthors(ctx, ctx.Repo.Repository, timeFrom, timeUntil, 10); err != nil {
		ctx.ServerError("GetActivityStatsTopAuthors", err)
		return
	}

	ctx.HTML(http.StatusOK, tplActivity)
}

// ActivityAuthors renders JSON with top commit authors for given time period over all branches
func ActivityAuthors(ctx *context.Context) {
	_, timeFrom, timeUntil, _ := activityDateRange(ctx.PathParam("period"), ctx.FormString("from"), ctx.FormString("until"), time.Now())

	authors, err := activities_model.GetActivityStatsTopAuthors(ctx, ctx.Repo.Repository, timeFrom, timeUntil, 10)
	if err != nil {
		ctx.ServerError("GetActivityStatsTopAuthors", err)
		return
	}

	ctx.JSON(http.StatusOK, authors)
}
