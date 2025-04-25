// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	goctx "context"
	"errors"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/context"
)

// StopwatchTmplInfo is a view on a stopwatch specifically for template rendering
type StopwatchTmplInfo struct {
	IssueLink  string
	RepoSlug   string
	IssueIndex int64
	Seconds    int64
}

func getActiveStopwatch(goCtx goctx.Context) *StopwatchTmplInfo {
	ctx := context.GetWebContext(goCtx)
	if ctx.Doer == nil {
		return nil
	}

	_, sw, issue, err := issues_model.HasUserStopwatch(ctx, ctx.Doer.ID)
	if err != nil {
		if !errors.Is(err, goctx.Canceled) {
			log.Error("Unable to HasUserStopwatch for user:%-v: %v", ctx.Doer, err)
		}
		return nil
	}

	if sw == nil || sw.ID == 0 {
		return nil
	}

	return &StopwatchTmplInfo{
		issue.Link(),
		issue.Repo.FullName(),
		issue.Index,
		sw.Seconds() + 1, // ensure time is never zero in ui
	}
}

func notificationUnreadCount(goCtx goctx.Context) int64 {
	ctx := context.GetWebContext(goCtx)
	if ctx.Doer == nil {
		return 0
	}
	count, err := db.Count[activities_model.Notification](ctx, activities_model.FindNotificationOptions{
		UserID: ctx.Doer.ID,
		Status: []activities_model.NotificationStatus{activities_model.NotificationStatusUnread},
	})
	if err != nil {
		if !errors.Is(err, goctx.Canceled) {
			log.Error("Unable to find notification for user:%-v: %v", ctx.Doer, err)
		}
		return 0
	}
	return count
}

func PageTmplFunctions(ctx *context.Context) {
	if ctx.IsSigned {
		// defer the function call to the last moment when the tmpl renders
		ctx.Data["NotificationUnreadCount"] = notificationUnreadCount
		ctx.Data["GetActiveStopwatch"] = getActiveStopwatch
	}
}
