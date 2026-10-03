// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package pubsub fans real-time events out to local WebSocket subscribers.
// Backend is chosen at boot: in-process map (single-instance) or Redis
// (multi-process). DefaultBroker is wired by Init from setting.Websocket.
package pubsub

import "fmt"

// subChanBuffer is how many messages a subscriber may fall behind before its
// messages are dropped instead of stalling the publisher.
const subChanBuffer = 8

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

// DefaultBroker is replaced by Init from setting.Websocket. It starts as an
// empty memory broker so non-web entry points (e.g. CLI), which skip Init, can
// publish without nil checks — with no subscribers every publish is a no-op.
// Tests construct a broker explicitly (NewMemoryBroker) instead of relying on this.
var DefaultBroker Broker = NewMemoryBroker()

func UserTopic(userID int64) string {
	return fmt.Sprintf("user-%d", userID)
}
