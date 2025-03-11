// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/services/context"
)

// IssueStopwatch creates or stops a stopwatch for the given issue.
func IssueStopwatch(c *context.Context) {
	issue := GetActionIssue(c)
	if c.Written() {
		return
	}

	var showSuccessMessage bool

	if !issues_model.StopwatchExists(c, c.Doer.ID, issue.ID) {
		showSuccessMessage = true
	}

	if !c.Repo.CanUseTimetracker(c, issue, c.Doer) {
		c.NotFound(nil)
		return
	}

	if err := issues_model.CreateOrStopIssueStopwatch(c, c.Doer, issue); err != nil {
		c.ServerError("CreateOrStopIssueStopwatch", err)
		return
	}

	if showSuccessMessage {
		c.Flash.Success(c.Tr("repo.issues.tracker_auto_close"))
	}

	c.JSONRedirect("")
}

// CancelStopwatch cancel the stopwatch
func CancelStopwatch(c *context.Context) {
	issue := GetActionIssue(c)
	if c.Written() {
		return
	}
	if !c.Repo.CanUseTimetracker(c, issue, c.Doer) {
		c.NotFound(nil)
		return
	}

	if err := issues_model.CancelStopwatch(c, c.Doer, issue); err != nil {
		c.ServerError("CancelStopwatch", err)
		return
	}

	stopwatches, err := issues_model.GetUserStopwatches(c, c.Doer.ID, db.ListOptions{})
	if err != nil {
		c.ServerError("GetUserStopwatches", err)
		return
	}
	if len(stopwatches) == 0 {
		eventsource.GetManager().SendMessage(c.Doer.ID, &eventsource.Event{
			Name: "stopwatches",
			Data: "{}",
		})
	}

	c.JSONRedirect("")
}

// GetActiveStopwatch is the middleware that sets .ActiveStopwatch on context
func GetActiveStopwatch(ctx *context.Context) {
	if strings.HasPrefix(ctx.Req.URL.Path, "/api") {
		return
	}

	if !ctx.IsSigned {
		return
	}

	_, sw, issue, err := issues_model.HasUserStopwatch(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("HasUserStopwatch", err)
		return
	}

	if sw == nil || sw.ID == 0 {
		return
	}

	ctx.Data["ActiveStopwatch"] = StopwatchTmplInfo{
		issue.Link(),
		issue.Repo.FullName(),
		issue.Index,
		sw.Seconds() + 1, // ensure time is never zero in ui
	}
}

// StopwatchTmplInfo is a view on a stopwatch specifically for template rendering
type StopwatchTmplInfo struct {
	IssueLink  string
	RepoSlug   string
	IssueIndex int64
	Seconds    int64
}
