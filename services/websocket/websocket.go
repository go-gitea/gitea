// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"bytes"
	"context"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	notify_service "gitea.dev/services/notify"
	"gitea.dev/services/pubsub"
)

type wsNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &wsNotifier{}

func Init() error {
	// the pubsub broker must be ready before the notifier starts publishing to it
	if err := pubsub.Init(); err != nil {
		return err
	}
	notify_service.RegisterNotifier(&wsNotifier{})
	return nil
}

func SubscribeUser(userID int64) (<-chan []byte, func()) {
	return pubsub.DefaultBroker.Subscribe(pubsub.UserTopic(userID))
}

type userDataEvent func(ctx context.Context, user *user_model.User) []byte

var userDataEvents []userDataEvent

func registerUserData(build userDataEvent) { userDataEvents = append(userDataEvents, build) }

// UserData is what a client receives on connect, so it starts in sync.
func UserData(ctx context.Context, user *user_model.User) (data [][]byte) {
	for _, build := range userDataEvents {
		if event := build(ctx, user); event != nil {
			data = append(data, event)
		}
	}
	return data
}

// EventForSession returns the JSON for a connection, nil if targeted elsewhere.
func EventForSession(brokerPayload []byte, sessionID string) []byte {
	target, eventData, _ := bytes.Cut(brokerPayload, []byte("\n"))
	if len(target) > 0 && string(target) != sessionID {
		return nil
	}
	return eventData
}

// makeUserEventMessage encodes a broker payload for every connection of a user.
func makeUserEventMessage(eventType string, eventData any) []byte {
	return makeSessionEventMessage(eventType, "", eventData)
}

// makeSessionEventMessage encodes target session, newline, client JSON.
func makeSessionEventMessage(eventType, targetSession string, eventData any) []byte {
	eventJSON, err := json.Marshal(&struct {
		EventType string `json:"eventType"`
		EventData any    `json:"eventData,omitzero"`
	}{eventType, eventData})
	if err != nil {
		setting.PanicInDevOrTesting("websocket: marshal event: %v", err)
		return nil
	}
	return append([]byte(targetSession+"\n"), eventJSON...)
}

// publishUserEvent publishes to a user, building only if someone is subscribed.
func publishUserEvent(userID int64, build func() []byte) {
	topic := pubsub.UserTopic(userID)
	if !pubsub.DefaultBroker.HasTopicSubscribers(topic) {
		return
	}
	if event := build(); event != nil {
		pubsub.DefaultBroker.Publish(topic, event)
	}
}
