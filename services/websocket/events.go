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

// UserEventMessage is the message the browser receives.
type UserEventMessage struct {
	EventType string `json:"eventType"`
	EventData any    `json:"eventData"` // no omitempty: an empty stopwatch list must stay an empty array
}

// BrokerMessage is what travels through the broker: the client payload plus the
// routing a connection needs to decide whether the payload is for it. Routing
// stays out of Payload so it never reaches the browser.
type BrokerMessage struct {
	// TargetSessionID limits delivery to one session of the user; empty means all of them.
	TargetSessionID string     `json:"targetSessionID,omitempty"`
	Payload         json.Value `json:"payload"`
}

// MakeBrokerMessage encodes an event for the broker, returning nil if it cannot be marshalled.
func MakeBrokerMessage(targetSessionID, eventType string, eventData any) []byte {
	payload, err := json.Marshal(&UserEventMessage{EventType: eventType, EventData: eventData})
	if err != nil {
		setting.PanicInDevOrTesting("websocket: marshal %q event: %v", eventType, err)
		return nil
	}
	b, err := json.Marshal(&BrokerMessage{TargetSessionID: targetSessionID, Payload: payload})
	if err != nil {
		setting.PanicInDevOrTesting("websocket: marshal broker message: %v", err)
		return nil
	}
	return b
}

// publishUserEvent sends an event to every session of the user.
func publishUserEvent(userID int64, eventType string, eventData any) {
	publishSessionEvent(userID, "", eventType, eventData)
}

// publishSessionEvent sends an event to one session of the user, or to all of them when sessionID is empty.
func publishSessionEvent(userID int64, sessionID, eventType string, eventData any) {
	if b := MakeBrokerMessage(sessionID, eventType, eventData); b != nil {
		pubsub.DefaultBroker.Publish(pubsub.UserTopic(userID), b)
	}
}
