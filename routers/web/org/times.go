// Copyright 2022 The Gitea Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"
	"time"

	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplByRepos      base.TplName = "org/times/times_by_repos"
	tplByMembers    base.TplName = "org/times/times_by_members"
	tplByMilestones base.TplName = "org/times/times_by_milestones"
)

// parseOrgTimes contains functionality that is required in all these functions,
// like parsing the date from the request, setting default dates, etc.
func parseOrgTimes(ctx *context.Context) (unixfrom, unixto int64, err error) {
	// Time range from request, if any
	from := ctx.FormString("from")
	to := ctx.FormString("to")
	// Defaults for "from" and "to" dates, if not in request
	if from == "" {
		// DEFAULT of "from": start of current month
		from = time.Now().Format("2006-01") + "-01"
	}
	if to == "" {
		// DEFAULT of "to": today
		to = time.Now().Format("2006-01-02")
	}

	// Prepare Form values
	ctx.Data["RangeFrom"] = from
	ctx.Data["RangeTo"] = to

	// Prepare unix time values for SQL
	from2, err := time.Parse("2006-01-02", from)
	if err != nil {
		ctx.ServerError("time.Parse", err)
	}
	unixfrom = from2.Unix()
	to2, err := time.Parse("2006-01-02", to)
	if err != nil {
		ctx.ServerError("time.Parse", err)
	}
	// Humans expect that we include the ending day too
	unixto = to2.Add(1440*time.Minute - 1*time.Second).Unix()
	return unixfrom, unixto, err
}

// TimesByRepos renders worktime by repositories.
func TimesByRepos(ctx *context.Context) {
	// Run common functionality
	unixfrom, unixto, err := parseOrgTimes(ctx)
	if err != nil {
		return
	}

	// View variables
	ctx.Data["PageIsOrgTimes"] = true
	ctx.Data["AppSubURL"] = setting.AppSubURL

	// Set submenu tab
	ctx.Data["TabIsByRepos"] = true

	results, err := organization.GetTimesByRepos(ctx.Org.Organization, unixfrom, unixto)
	if err != nil {
		ctx.ServerError("getTimesByRepos", err)
		return
	}
	ctx.Data["results"] = results

	// Reply with view
	ctx.HTML(http.StatusOK, tplByRepos)
}

// TimesByMilestones renders work time by milestones.
func TimesByMilestones(ctx *context.Context) {
	// Run common functionality
	unixfrom, unixto, err := parseOrgTimes(ctx)
	if err != nil {
		return
	}

	// View variables
	ctx.Data["PageIsOrgTimes"] = true
	ctx.Data["AppSubURL"] = setting.AppSubURL

	// Set submenu tab
	ctx.Data["TabIsByMilestones"] = true

	// Get the data from the DB
	results, err := organization.GetTimesByMilestones(ctx.Org.Organization, unixfrom, unixto)
	if err != nil {
		ctx.ServerError("getTimesByMilestones", err)
		return
	}

	// Show only the first RepoName, for nicer output.
	prevreponame := ""
	for i := 0; i < len(results); i++ {
		res := &results[i]
		if prevreponame == res.RepoName {
			res.HideRepoName = true
		}
		prevreponame = res.RepoName
	}

	// Send results to view
	ctx.Data["results"] = results

	// Reply with view
	ctx.HTML(http.StatusOK, tplByMilestones)
}

// TimesByMembers renders worktime by project member persons.
func TimesByMembers(ctx *context.Context) {
	// Run common functionality
	unixfrom, unixto, err := parseOrgTimes(ctx)
	if err != nil {
		return
	}

	// View variables
	ctx.Data["PageIsOrgTimes"] = true
	ctx.Data["AppSubURL"] = setting.AppSubURL

	// Set submenu tab
	ctx.Data["TabIsByMembers"] = true

	// Get the data from the DB
	results, err := organization.GetTimesByMembers(ctx.Org.Organization, unixfrom, unixto)
	if err != nil {
		ctx.ServerError("getTimesByMembers", err)
		return
	}
	ctx.Data["results"] = results

	ctx.HTML(http.StatusOK, tplByMembers)
}
