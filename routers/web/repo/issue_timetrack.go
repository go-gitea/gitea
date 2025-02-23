// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	issue_service "code.gitea.io/gitea/services/issue"
)

// AddTimeManually tracks time manually
func AddTimeManually(c *context.Context) {
	form := web.GetForm(c).(*forms.AddTimeManuallyForm)
	issue := GetActionIssue(c)
	if c.Written() {
		return
	}
	if !c.Repo.CanUseTimetracker(c, issue, c.Doer) {
		c.NotFound(nil)
		return
	}

	if c.HasError() {
		c.JSONError(c.GetErrMsg())
		return
	}

	total := time.Duration(form.Hours)*time.Hour + time.Duration(form.Minutes)*time.Minute

	if total <= 0 {
		c.JSONError(c.Tr("repo.issues.add_time_sum_to_small"))
		return
	}

	if _, err := issues_model.AddTime(c, c.Doer, issue, int64(total.Seconds()), time.Now()); err != nil {
		c.ServerError("AddTime", err)
		return
	}

	c.JSONRedirect("")
}

// DeleteTime deletes tracked time
func DeleteTime(c *context.Context) {
	issue := GetActionIssue(c)
	if c.Written() {
		return
	}
	if !c.Repo.CanUseTimetracker(c, issue, c.Doer) {
		c.NotFound(nil)
		return
	}

	t, err := issues_model.GetTrackedTimeByID(c, c.PathParamInt64("timeid"))
	if err != nil {
		if db.IsErrNotExist(err) {
			c.NotFound(err)
			return
		}
		c.HTTPError(http.StatusInternalServerError, "GetTrackedTimeByID", err.Error())
		return
	}

	// only OP or admin may delete
	if !c.IsSigned || (!c.IsUserSiteAdmin() && c.Doer.ID != t.UserID) {
		c.HTTPError(http.StatusForbidden, "not allowed")
		return
	}

	if err = issues_model.DeleteTime(c, t); err != nil {
		c.ServerError("DeleteTime", err)
		return
	}

	c.Flash.Success(c.Tr("repo.issues.del_time_history", util.SecToHours(t.Time)))
	c.JSONRedirect("")
}

func UpdateIssueTimeEstimate(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (!issue.IsPoster(ctx.Doer.ID) && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	timeStr := strings.TrimSpace(ctx.FormString("time_estimate"))

	total, err := util.TimeEstimateParse(timeStr)
	if err != nil {
		ctx.JSONError(ctx.Tr("repo.issues.time_estimate_invalid"))
		return
	}

	// No time changed
	if issue.TimeEstimate == total {
		ctx.JSONRedirect("")
		return
	}

	if err := issue_service.ChangeTimeEstimate(ctx, issue, ctx.Doer, total); err != nil {
		ctx.ServerError("ChangeTimeEstimate", err)
		return
	}

	ctx.JSONRedirect("")
}
