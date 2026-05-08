// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import (
	"fmt"
	"sync"

	"code.gitea.io/gitea/modules/log"
)

type Broker struct {
	mu   sync.RWMutex
	subs map[string][]chan []byte
}

var DefaultBroker = NewBroker()

func NewBroker() *Broker {
	return &Broker{
		subs: make(map[string][]chan []byte),
	}
}

func (b *Broker) Subscribe(topic string) (<-chan []byte, func()) {
	ch := make(chan []byte, 8)

	b.mu.Lock()
	b.subs[topic] = append(b.subs[topic], ch)
	b.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			subs := b.subs[topic]
			for i, sub := range subs {
				if sub == ch {
					subs = append(subs[:i], subs[i+1:]...)
					break
				}
			}
			if len(subs) == 0 {
				delete(b.subs, topic)
			} else {
				b.subs[topic] = subs
			}
			close(ch)
		})
	}
	return ch, cancel
}

func UserTopic(userID int64) string {
	return fmt.Sprintf("user-%d", userID)
}

func (b *Broker) HasTopicSubscribers(topic string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs[topic]) > 0
}

// Non-blocking: slow subscribers drop. RLock held across fan-out to block
// cancel() from closing a channel between slice read and send.
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
