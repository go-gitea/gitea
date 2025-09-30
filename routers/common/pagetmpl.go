// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	goctx "context"
	"errors"
	"sync"

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

func getActiveStopwatch(ctx *context.Context) *StopwatchTmplInfo {
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

func notificationUnreadCount(ctx *context.Context) int64 {
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

type pageGlobalDataType struct {
	IsSigned    bool
	IsSiteAdmin bool

	GetNotificationUnreadCount func() int64
	GetActiveStopwatch         func() *StopwatchTmplInfo
}

func PageGlobalData(ctx *context.Context) {
	var data pageGlobalDataType
	data.IsSigned = ctx.Doer != nil
	data.IsSiteAdmin = ctx.Doer != nil && ctx.Doer.IsAdmin
	data.GetNotificationUnreadCount = sync.OnceValue(func() int64 { return notificationUnreadCount(ctx) })
	data.GetActiveStopwatch = sync.OnceValue(func() *StopwatchTmplInfo { return getActiveStopwatch(ctx) })
	ctx.Data["PageGlobalData"] = data
}
