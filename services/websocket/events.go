// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/services/pubsub"
)

// Wire contract with web_src/js/user-events.sharedworker.ts — keep in sync.
const (
	EventNotificationCount = "notification-count"
	EventStopwatches       = "stopwatches"
	EventLogout            = "logout"
)

type UserEventMessage[T any] struct {
	EventType string `json:"eventType"`
	EventData T      `json:"eventData"`
}

func publishUserEvent(userID int64, eventType string, eventData any) {
	b := MakeUserEventMessage(eventType, eventData)
	if b == nil {
		return
	}
	pubsub.DefaultBroker.Publish(pubsub.UserTopic(userID), b)
}

// MakeUserEventMessage encodes an event for the broker. The payload reaches the
// browser verbatim, so it is plain JSON with no extra framing.
func MakeUserEventMessage(eventType string, eventData any) []byte {
	b, err := json.Marshal(&UserEventMessage[any]{EventType: eventType, EventData: eventData})
	if err != nil {
		setting.PanicInDevOrTesting("websocket: marshal event: %v", err)
		return nil
	}
	return b
}
