// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"

	activities_model "gitea.dev/models/activities"
	"gitea.dev/models/db"
	"gitea.dev/modules/log"
	notify_service "gitea.dev/services/notify"
	"gitea.dev/services/pubsub"
)

type notificationCountEvent struct {
	Type  string `json:"type"`
	Count int64  `json:"count"`
}

type wsNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &wsNotifier{}

func (n *wsNotifier) NotificationCountChange(ctx context.Context, userID int64) {
	if !pubsub.DefaultBroker.HasTopicSubscribers(pubsub.UserTopic(userID)) {
		return
	}
	count, err := db.Count[activities_model.Notification](ctx, activities_model.FindNotificationOptions{
		UserID: userID,
		Status: []activities_model.NotificationStatus{activities_model.NotificationStatusUnread},
	})
	if err != nil {
		log.Error("websocket: count notifications for user %d: %v", userID, err)
		return
	}
	publishUserEvent(userID, notificationCountEvent{
		Type:  EventNotificationCount,
		Count: count,
	})
}
