// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/pubsub"
)

// Wire contract with web_src/js/user-events.sharedworker.ts — keep in sync.
const (
	EventNotificationCount = "notification-count"
	EventStopwatches       = "stopwatches"
	EventLogout            = "logout"
)

func publishUserEvent(userID int64, event any) {
	msg, err := json.Marshal(event)
	if err != nil {
		log.Error("websocket: marshal event: %v", err)
		return
	}
	pubsub.DefaultBroker.Publish(pubsub.UserTopic(userID), msg)
}
