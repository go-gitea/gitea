// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/services/context"
)

// IssueStartStopwatch creates a stopwatch for the given issue.
func IssueStartStopwatch(c *context.Context) {
	issue := GetActionIssue(c)
	if c.Written() {
		return
	}

	if !c.Repo.CanUseTimetracker(c, issue, c.Doer) {
		c.NotFound(nil)
		return
	}

	if ok, err := issues_model.CreateIssueStopwatch(c, c.Doer, issue); err != nil {
		c.ServerError("CreateIssueStopwatch", err)
		return
	} else if !ok {
		c.Flash.Warning(c.Tr("repo.issues.stopwatch_already_created"))
	} else {
		c.Flash.Success(c.Tr("repo.issues.tracker_auto_close"))
	}
	c.JSONRedirect("")
}

// IssueStopStopwatch stops a stopwatch for the given issue.
func IssueStopStopwatch(c *context.Context) {
	issue := GetActionIssue(c)
	if c.Written() {
		return
	}

	if !c.Repo.CanUseTimetracker(c, issue, c.Doer) {
		c.NotFound(nil)
		return
	}

	if ok, err := issues_model.FinishIssueStopwatch(c, c.Doer, issue); err != nil {
		c.ServerError("FinishIssueStopwatch", err)
		return
	} else if !ok {
		c.Flash.Warning(c.Tr("repo.issues.stopwatch_already_stopped"))
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

	if _, err := issues_model.CancelStopwatch(c, c.Doer, issue); err != nil {
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
