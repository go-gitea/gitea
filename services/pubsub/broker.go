// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package pubsub fans real-time events out to local WebSocket subscribers.
// Backend is chosen at boot: in-process map (single-instance) or Redis
// (multi-process). DefaultBroker is wired by Init from setting.Websocket.
package pubsub

import "fmt"

type Broker interface {
	// Subscribe returns a buffered channel of messages for topic, and a cancel
	// func that closes the channel and removes the subscription. cancel is
	// idempotent.
	Subscribe(topic string) (<-chan []byte, func())

	// Publish delivers msg to every subscriber of topic. Non-blocking: a slow
	// subscriber drops messages rather than stalling the publisher.
	Publish(topic string, msg []byte)

	// HasTopicSubscribers is an optimization hint for publishers that would
	// otherwise do a DB lookup just to discover nobody is listening. Backends
	// that cannot answer cheaply across processes return true to be safe.
	HasTopicSubscribers(topic string) bool
}

// DefaultBroker is set by Init. Tests and standalone callers that need a
// broker without Init can assign a fresh MemoryBroker.
var DefaultBroker Broker = NewMemoryBroker()

func UserTopic(userID int64) string {
	return fmt.Sprintf("user-%d", userID)
}
