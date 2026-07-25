// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"bytes"

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
	if pubsub.DefaultBroker == nil {
		return
	}
	b := MakeUserEventMessage(eventType, eventData)
	if b == nil {
		return
	}
	pubsub.DefaultBroker.Publish(pubsub.UserTopic(userID), b)
}

func MakeUserEventMessage(eventType string, eventData any) []byte {
	buf := &bytes.Buffer{}
	buf.WriteString(eventType)
	buf.WriteByte('\n')
	err := json.MarshalWrite(buf, &UserEventMessage[any]{EventType: eventType, EventData: eventData})
	if err != nil {
		setting.PanicInDevOrTesting("websocket: marshal event: %v", err)
		return nil
	}
	return buf.Bytes()
}

func ExtractUserEventMessage(b []byte) (string, []byte) {
	eventType, eventDataBytes, _ := bytes.Cut(b, []byte("\n"))
	return string(eventType), eventDataBytes
}
