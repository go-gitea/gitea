// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import (
	"context"
	"sync"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/nosql"

	"github.com/redis/go-redis/v9"
)

// RedisBroker fans out across processes via Redis pub/sub. Each topic is
// backed by a single Redis SUBSCRIBE shared between local subscribers; the
// last local Unsubscribe tears the Redis subscription down.
type RedisBroker struct {
	client redis.UniversalClient

	mu     sync.Mutex
	topics map[string]*redisTopic
}

type redisTopic struct {
	ps     *redis.PubSub
	subs   []chan []byte
	cancel context.CancelFunc
}

var _ Broker = (*RedisBroker)(nil)

func NewRedisBroker(connStr string) (*RedisBroker, error) {
	client := nosql.GetManager().GetRedisClient(connStr)
	if err := client.Ping(graceful.GetManager().ShutdownContext()).Err(); err != nil {
		return nil, err
	}
	return &RedisBroker{
		client: client,
		topics: make(map[string]*redisTopic),
	}, nil
}

func (b *RedisBroker) Subscribe(topic string) (<-chan []byte, func()) {
	ch := make(chan []byte, 8)

	b.mu.Lock()
	state, exists := b.topics[topic]
	if !exists {
		// graceful.ShutdownContext so the reader loop dies cleanly on Gitea shutdown
		// even if every local subscriber has already cancelled.
		ctx, cancel := context.WithCancel(graceful.GetManager().ShutdownContext())
		state = &redisTopic{
			ps:     b.client.Subscribe(ctx, topic),
			cancel: cancel,
		}
		b.topics[topic] = state
		go b.readLoop(ctx, topic, state.ps)
	}
	state.subs = append(state.subs, ch)
	b.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			subs := state.subs
			for i, sub := range subs {
				if sub == ch {
					subs = append(subs[:i], subs[i+1:]...)
					break
				}
			}
			state.subs = subs
			if len(subs) == 0 {
				state.cancel()
				_ = state.ps.Close()
				delete(b.topics, topic)
			}
			close(ch)
		})
	}
	return ch, cancel
}

func (b *RedisBroker) readLoop(ctx context.Context, topic string, ps *redis.PubSub) {
	for {
		msg, err := ps.ReceiveMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Trace("pubsub redis: receive on %q: %v", topic, err)
			return
		}
		payload := []byte(msg.Payload)
		b.mu.Lock()
		state, ok := b.topics[topic]
		if !ok {
			b.mu.Unlock()
			return
		}
		for _, ch := range state.subs {
			select {
			case ch <- payload:
			default:
				log.Trace("pubsub redis: dropping message on topic %q — subscriber channel full", topic)
			}
		}
		b.mu.Unlock()
	}
}

func (b *RedisBroker) Publish(topic string, msg []byte) {
	if err := b.client.Publish(graceful.GetManager().HammerContext(), topic, msg).Err(); err != nil {
		log.Error("pubsub redis: publish to %q: %v", topic, err)
	}
}

// HasTopicSubscribers conservatively returns true: cross-process subscriber
// discovery via PUBSUB NUMSUB is per-node and would silently miss subscribers
// in cluster mode. Publishers do the upstream lookup unconditionally.
func (b *RedisBroker) HasTopicSubscribers(topic string) bool {
	return true
}
