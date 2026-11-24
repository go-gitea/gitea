// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"

	"gitea.dev/modules/json"
	"gitea.dev/services/pubsub"
	websocket_service "gitea.dev/services/websocket"

	gitea_ws "github.com/coder/websocket"
)

const maxTopicSubscriptions = 64

type clientMessage struct { // keep in sync with web_src/js/user-events.sharedworker.ts
	Type  string `json:"type"`
	Topic string `json:"topic"`
}

func readClientMessages(ctx context.Context, cancelConnection context.CancelFunc, conn *gitea_ws.Conn, doerID int64, canSubscribe bool, send func([]byte) error) {
	defer cancelConnection()
	subscriptions := map[string]context.CancelFunc{}
	for {
		messageType, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var msg clientMessage
		if !canSubscribe || messageType != gitea_ws.MessageText || json.Unmarshal(data, &msg) != nil || (msg.Type != "subscribe" && msg.Type != "unsubscribe") {
			continue
		}
		if cancel, ok := subscriptions[msg.Topic]; ok {
			cancel()
			delete(subscriptions, msg.Topic)
		}
		if msg.Type == "unsubscribe" || len(subscriptions) >= maxTopicSubscriptions {
			continue
		}
		if topic := websocket_service.GetDoerTopic(ctx, doerID, msg.Topic); topic != nil {
			topicCtx, cancel := context.WithCancel(ctx)
			subscriptions[msg.Topic] = cancel
			go streamTopic(topicCtx, cancelConnection, msg.Topic, topic, send)
		}
	}
}

func streamTopic(ctx context.Context, cancelConnection context.CancelFunc, name string, topic websocket_service.Topic, send func([]byte) error) {
	defer func() {
		if ctx.Err() == nil {
			cancelConnection() // a failed stream resets the connection, the client then resubscribes with fresh state
		}
	}()
	ch, unsubscribe := pubsub.DefaultBroker.Subscribe(name)
	defer unsubscribe()
	for {
		if send(topic.CurrentEvent(ctx)) != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ch:
			if !ok || !topic.CanRead(ctx) {
				return
			}
		}
	}
}
