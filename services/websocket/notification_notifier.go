// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	notify_service "code.gitea.io/gitea/services/notify"
	"code.gitea.io/gitea/services/pubsub"
)

type notificationCountEvent struct {
	Type  string `json:"type"`
	Count int64  `json:"count"`
}

type wsNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &wsNotifier{}

// NotificationCountChange queries the current unread count for the user and
// pushes it to all connected WebSocket clients. Skips the DB lookup when the
// user has no active subscribers.
func (n *wsNotifier) NotificationCountChange(ctx context.Context, userID int64) {
	topic := pubsub.UserTopic(userID)
	if !pubsub.DefaultBroker.HasTopicSubscribers(topic) {
		return
	}
	count, err := db.Count[activities_model.Notification](ctx, activities_model.FindNotificationOptions{
		UserID: userID,
		Status: []activities_model.NotificationStatus{activities_model.NotificationStatusUnread},
	})
	if err != nil {
		log.Error("websocket: NotificationCountChange count %d: %v", userID, err)
		return
	}
	msg, err := json.Marshal(notificationCountEvent{
		Type:  EventNotificationCount,
		Count: count,
	})
	if err != nil {
		log.Error("websocket: marshal notification-count event: %v", err)
		return
	}
	pubsub.DefaultBroker.Publish(topic, msg)
}
