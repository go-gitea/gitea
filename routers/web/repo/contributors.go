// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
	repo_service "gitea.dev/services/repository"
)

const (
	tplContributors templates.TplName = "repo/activity"
)

// Contributors render the page to show repository contributors graph
func Contributors(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.activity.navbar.contributors")
	ctx.Data["PageIsActivity"] = true
	ctx.Data["PageIsContributors"] = true
	ctx.HTML(http.StatusOK, tplContributors)
}

// ContributorsData renders JSON of contributors along with their weekly commit statistics
func ContributorsData(ctx *context.Context) {
	var start *time.Time
	var end *time.Time
	if from := ctx.FormString("from"); from != "" {
		value, err := strconv.ParseInt(from, 10, 64)
		if err != nil {
			ctx.Status(http.StatusBadRequest)
			return
		}
		timeValue := time.UnixMilli(value).UTC()
		start = &timeValue
	}
	if to := ctx.FormString("to"); to != "" {
		value, err := strconv.ParseInt(to, 10, 64)
		if err != nil {
			ctx.Status(http.StatusBadRequest)
			return
		}
		timeValue := time.UnixMilli(value).UTC()
		end = &timeValue
	}
	if start != nil && end != nil && !start.Before(*end) {
		ctx.Status(http.StatusBadRequest)
		return
	}

	if contributorStats, err := repo_service.GetContributorStats(ctx, ctx.Repo.Repository, 100, start, end); err != nil {
		if errors.Is(err, repo_service.ErrAwaitGeneration) {
			ctx.Status(http.StatusAccepted)
			return
		}
		ctx.ServerError("GetContributorStats", err)
	} else {
		ctx.JSON(http.StatusOK, contributorStats)
	}
}
