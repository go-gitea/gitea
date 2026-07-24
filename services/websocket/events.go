// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/services/pubsub"
)

// Wire contract with web_src/js/user-events.sharedworker.ts — keep in sync.
const (
	EventNotificationCount = "notification-count"
	EventStopwatches       = "stopwatches"
	EventLogout            = "logout"
)

// userEvent is the {type, data} envelope for data-carrying client events. Go
// generics let one type serialize any payload; the client (types.ts)
// discriminates on Type. Deliberately no omitempty: an empty payload must still
// serialize its data key (e.g. stopwatches sends "data":[] when the list is
// empty). Events with a non-"data" shape (notification-count's flat "count") or
// no data (logout) are not this envelope and stay their own types.
type userEvent[T any] struct {
	Type string `json:"type"`
	Data T      `json:"data"`
}

func publishUserEvent(userID int64, event any) {
	// nil when Init was skipped (e.g. CLI user delete): no web subscribers exist, nothing to publish.
	if pubsub.DefaultBroker == nil {
		return
	}
	msg, err := json.Marshal(event)
	if err != nil {
		log.Error("websocket: marshal event: %v", err)
		return
	}
	pubsub.DefaultBroker.Publish(pubsub.UserTopic(userID), msg)
}
