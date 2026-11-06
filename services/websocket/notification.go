// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"

	activities_model "gitea.dev/models/activities"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
)

const eventNotificationCount = "notification-count" // keep in sync with web_src/js/types.ts

func (n *wsNotifier) NotificationCountChange(ctx context.Context, userID int64) {
	publishUserEvent(userID, func() []byte { return notificationCountEvent(ctx, userID) })
}

func notificationCountEvent(ctx context.Context, userID int64) []byte {
	viewer, err := user_model.GetUserByID(ctx, userID)
	if err != nil {
		log.Error("websocket: load user %d: %v", userID, err)
		return nil
	}
	opts := activities_model.FindNotificationOptions{
		UserID: userID,
		Viewer: viewer,
		Status: []activities_model.NotificationStatus{activities_model.NotificationStatusUnread},
	}
	var count int64
	if err = opts.ResolveHiddenIssues(ctx); err == nil {
		count, err = db.Count[activities_model.Notification](ctx, opts)
	}
	if err != nil {
		log.Error("websocket: count notifications for user %d: %v", userID, err)
		return nil
	}
	return makeEventMessage(eventNotificationCount, "", struct {
		Count int64 `json:"count"`
	}{count})
}
