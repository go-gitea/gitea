// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import (
	"sync"
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

// Publish sends msg to all subscribers of topic.
// Non-blocking: slow subscribers are skipped.
func (b *Broker) Publish(topic string, msg []byte) {
	b.mu.RLock()
	subs := b.subs[topic]
	b.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- msg:
		default:
			// subscriber too slow — skip
		}
	}
}
