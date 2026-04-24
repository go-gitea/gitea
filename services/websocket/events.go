// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/pubsub"
)

// Wire-format event type names. These strings are the contract between the Go
// publishers and the TypeScript shared worker in web_src/js/user-events.sharedworker.ts
// — keep both sides in sync.
const (
	EventNotificationCount = "notification-count"
	EventStopwatches       = "stopwatches"
	EventLogout            = "logout"
)

// publishUserEvent marshals the event to JSON and publishes it on the user's topic.
func publishUserEvent(userID int64, eventName string, event any) {
	msg, err := json.Marshal(event)
	if err != nil {
		log.Error("websocket: marshal %s event: %v", eventName, err)
		return
	}
	pubsub.DefaultBroker.Publish(pubsub.UserTopic(userID), msg)
}
