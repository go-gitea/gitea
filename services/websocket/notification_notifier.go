// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	notify_service "code.gitea.io/gitea/services/notify"
)

type wsNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &wsNotifier{}

// NotificationCountChange queries the current unread count for the user and
// pushes it immediately to all connected WebSocket clients, bypassing the
// periodic polling loop for this specific user.
func (n *wsNotifier) NotificationCountChange(ctx context.Context, userID int64) {
	count, err := db.Count[activities_model.Notification](ctx, activities_model.FindNotificationOptions{
		UserID: userID,
		Status: []activities_model.NotificationStatus{activities_model.NotificationStatusUnread},
	})
	if err != nil {
		log.Error("websocket: NotificationCountChange count %d: %v", userID, err)
		return
	}
	publishNotificationCount(userID, count)
}
