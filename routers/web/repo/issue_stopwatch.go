// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/eventsource"
)

// IssueStopwatch creates or stops a stopwatch for the given issue.
func IssueStopwatch(c *context.Context) {
	issue := GetActionIssue(c)
	if c.Written() {
		return
	}

	var showSuccessMessage bool

	if !models.StopwatchExists(c.Doer.ID, issue.ID) {
		showSuccessMessage = true
	}

	if !c.Repo.CanUseTimetracker(issue, c.Doer) {
		c.NotFound("CanUseTimetracker", nil)
		return
	}

	if err := models.CreateOrStopIssueStopwatch(c.Doer, issue); err != nil {
		c.ServerError("CreateOrStopIssueStopwatch", err)
		return
	}

	if showSuccessMessage {
		c.Flash.Success(c.Tr("repo.issues.tracker_auto_close"))
	}

	url := issue.HTMLURL()
	c.Redirect(url, http.StatusSeeOther)
}

// CancelStopwatch cancel the stopwatch
func CancelStopwatch(c *context.Context) {
	issue := GetActionIssue(c)
	if c.Written() {
		return
	}
	if !c.Repo.CanUseTimetracker(issue, c.Doer) {
		c.NotFound("CanUseTimetracker", nil)
		return
	}

	if err := models.CancelStopwatch(c.Doer, issue); err != nil {
		c.ServerError("CancelStopwatch", err)
		return
	}

	stopwatches, err := models.GetUserStopwatches(c.Doer.ID, db.ListOptions{})
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

	url := issue.HTMLURL()
	c.Redirect(url, http.StatusSeeOther)
}

// GetActiveStopwatch is the middleware that sets .ActiveStopwatch on context
func GetActiveStopwatch(ctx *context.Context) {
	if strings.HasPrefix(ctx.Req.URL.Path, "/api") {
		return
	}

	if !ctx.IsSigned {
		return
	}

	_, sw, err := models.HasUserStopwatch(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("HasUserStopwatch", err)
		return
	}

	if sw == nil || sw.ID == 0 {
		return
	}

	issue, err := models.GetIssueByID(sw.IssueID)
	if err != nil || issue == nil {
		ctx.ServerError("GetIssueByID", err)
		return
	}
	if err = issue.LoadRepo(ctx); err != nil {
		ctx.ServerError("LoadRepo", err)
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
