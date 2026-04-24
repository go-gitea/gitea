// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import (
	"fmt"
	"sync"

	"code.gitea.io/gitea/modules/log"
)

// Broker is a simple in-memory pub/sub broker.
// It supports fan-out: one Publish call delivers the message to all active subscribers.
type Broker struct {
	mu   sync.RWMutex
	subs map[string][]chan []byte
}

// DefaultBroker is the global singleton used by both routers and notifiers.
var DefaultBroker = NewBroker()

// NewBroker creates a new in-memory Broker.
func NewBroker() *Broker {
	return &Broker{
		subs: make(map[string][]chan []byte),
	}
}

// Subscribe returns a channel that receives messages published to topic.
// Call the returned cancel function to unsubscribe.
func (b *Broker) Subscribe(topic string) (<-chan []byte, func()) {
	ch := make(chan []byte, 8)

	b.mu.Lock()
	b.subs[topic] = append(b.subs[topic], ch)
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		subs := b.subs[topic]
		for i, sub := range subs {
			if sub == ch {
				b.subs[topic] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		close(ch)
	}
	return ch, cancel
}

// UserTopic returns the pub/sub topic name for a given user ID.
// Centralised here so the notifier and the WebSocket handler always agree on the format.
func UserTopic(userID int64) string {
	return fmt.Sprintf("user-%d", userID)
}

// HasSubscribers reports whether the broker has at least one active subscriber across all topics.
func (b *Broker) HasSubscribers() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, subs := range b.subs {
		if len(subs) > 0 {
			return true
		}
	}
	return false
}

// Publish sends msg to all subscribers of topic.
// Non-blocking: slow subscribers are skipped. The drop is logged at Trace
// level so persistent back-pressure is diagnosable without spamming prod logs.
// The RLock is held for the entire fan-out to prevent a race where cancel()
// closes a channel between the slice read and the send.
func (b *Broker) Publish(topic string, msg []byte) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subs[topic] {
		select {
		case ch <- msg:
		default:
			log.Trace("pubsub: dropping message on topic %q — subscriber channel full", topic)
		}
	}
}
