// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
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

	url := issue.HTMLURL()
	c.Redirect(url, http.StatusSeeOther)
}

// GetActiveStopwatch is the middleware that sets .ActiveStopwatch on context
func GetActiveStopwatch(c *context.Context) {
	if strings.HasPrefix(c.Req.URL.Path, "/api") {
		return
	}

	if !c.IsSigned {
		return
	}

	_, sw, err := models.HasUserStopwatch(c.Doer.ID)
	if err != nil {
		c.ServerError("HasUserStopwatch", err)
		return
	}

	if sw == nil || sw.ID == 0 {
		return
	}

	issue, err := models.GetIssueByID(sw.IssueID)
	if err != nil || issue == nil {
		c.ServerError("GetIssueByID", err)
		return
	}
	if err = issue.LoadRepo(); err != nil {
		c.ServerError("LoadRepo", err)
		return
	}

	c.Data["ActiveStopwatch"] = StopwatchTmplInfo{
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
