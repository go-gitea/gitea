// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import (
	"context"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/nosql"

	"github.com/redis/go-redis/v9"
)

const (
	redisPingTimeout      = 3 * time.Second
	redisSubscribeTimeout = 5 * time.Second
	redisPublishTimeout   = 2 * time.Second
)

// RedisBroker fans out across processes via Redis pub/sub. Each topic is
// backed by a single Redis SUBSCRIBE shared between local subscribers; the
// last local Unsubscribe tears the Redis subscription down.
type RedisBroker struct {
	client redis.UniversalClient

	mu     sync.RWMutex
	topics map[string]*redisTopic
}

type redisTopic struct {
	ps     *redis.PubSub
	subs   []*redisSub
	cancel context.CancelFunc
}

// redisSub pairs a delivery channel with the once that guards its close, so
// either cancel() or readLoop's error-exit path can safely close it.
type redisSub struct {
	ch   chan []byte
	once sync.Once
}

func (s *redisSub) close() { s.once.Do(func() { close(s.ch) }) }

var _ Broker = (*RedisBroker)(nil)

func NewRedisBroker(connStr string) (*RedisBroker, error) {
	client := nosql.GetManager().GetRedisClient(connStr)
	// context.Background not graceful.ShutdownContext: this runs during boot,
	// when ShutdownContext may not even be initialized yet, and would otherwise
	// fail immediately with a misleading "context canceled" on late init.
	pingCtx, cancel := context.WithTimeout(context.Background(), redisPingTimeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		return nil, err
	}
	return &RedisBroker{
		client: client,
		topics: make(map[string]*redisTopic),
	}, nil
}

func (b *RedisBroker) Subscribe(topic string) (<-chan []byte, func()) {
	sub := &redisSub{ch: make(chan []byte, 8)}

	// Fast path: topic already has a Redis subscription, just attach locally.
	b.mu.Lock()
	if state, exists := b.topics[topic]; exists {
		state.subs = append(state.subs, sub)
		b.mu.Unlock()
		return sub.ch, b.makeCancel(topic, sub)
	}
	b.mu.Unlock()

	// Slow path: create the Redis subscription outside the broker mutex so
	// other Subscribe/cancel calls aren't blocked on the network round-trip.
	// graceful.ShutdownContext so the reader loop dies cleanly on Gitea
	// shutdown even if every local subscriber has already cancelled.
	ctx, cancelCtx := context.WithCancel(graceful.GetManager().ShutdownContext())
	ps := b.client.Subscribe(ctx, topic)
	subscribeCtx, subscribeCancel := context.WithTimeout(ctx, redisSubscribeTimeout)
	// ps.Subscribe sends SUBSCRIBE but does not block for the server ack —
	// without this Receive a Publish that fires immediately after Subscribe
	// returns can outrun the server-side registration (notably miniredis in
	// tests, where there's no network latency to mask the race).
	_, err := ps.Receive(subscribeCtx)
	subscribeCancel()
	if err != nil {
		cancelCtx()
		_ = ps.Close()
		close(sub.ch)
		log.Error("pubsub redis: subscribe %q: %v", topic, err)
		return sub.ch, func() {}
	}

	b.mu.Lock()
	if existing, exists := b.topics[topic]; exists {
		// Another goroutine won the create race; merge into theirs and discard ours.
		existing.subs = append(existing.subs, sub)
		b.mu.Unlock()
		cancelCtx()
		_ = ps.Close()
		return sub.ch, b.makeCancel(topic, sub)
	}
	b.topics[topic] = &redisTopic{ps: ps, cancel: cancelCtx, subs: []*redisSub{sub}}
	b.mu.Unlock()
	go b.readLoop(ctx, topic, ps)
	return sub.ch, b.makeCancel(topic, sub)
}

func (b *RedisBroker) makeCancel(topic string, sub *redisSub) func() {
	return func() {
		b.mu.Lock()
		state, ok := b.topics[topic]
		if !ok {
			b.mu.Unlock()
			sub.close()
			return
		}
		for i, s := range state.subs {
			if s == sub {
				state.subs = append(state.subs[:i], state.subs[i+1:]...)
				break
			}
		}
		if len(state.subs) == 0 {
			state.cancel()
			_ = state.ps.Close()
			delete(b.topics, topic)
		}
		b.mu.Unlock()
		sub.close()
	}
}

func (b *RedisBroker) readLoop(ctx context.Context, topic string, ps *redis.PubSub) {
	for {
		msg, err := ps.ReceiveMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			// Transport blip: tear the topic down so a fresh Subscribe rebuilds it.
			// Closing each subscriber's channel wakes the WebSocket handler, which
			// will reconnect and re-Subscribe.
			b.mu.Lock()
			if cur, ok := b.topics[topic]; ok && cur.ps == ps {
				for _, s := range cur.subs {
					s.close()
				}
				cur.cancel()
				_ = cur.ps.Close()
				delete(b.topics, topic)
			}
			b.mu.Unlock()
			log.Trace("pubsub redis: receive on %q: %v", topic, err)
			return
		}
		payload := []byte(msg.Payload)
		b.mu.RLock()
		state, ok := b.topics[topic]
		if !ok {
			b.mu.RUnlock()
			return
		}
		for _, s := range state.subs {
			select {
			case s.ch <- payload:
			default:
				log.Trace("pubsub redis: dropping message on topic %q — subscriber channel full", topic)
			}
		}
		b.mu.RUnlock()
	}
}

func (b *RedisBroker) Publish(topic string, msg []byte) {
	ctx, cancel := context.WithTimeout(graceful.GetManager().HammerContext(), redisPublishTimeout)
	defer cancel()
	if err := b.client.Publish(ctx, topic, msg).Err(); err != nil {
		log.Error("pubsub redis: publish to %q: %v", topic, err)
	}
}

// HasTopicSubscribers conservatively returns true: cross-process subscriber
// discovery via PUBSUB NUMSUB is per-node and would silently miss subscribers
// in cluster mode. Publishers do the upstream lookup unconditionally.
func (b *RedisBroker) HasTopicSubscribers(topic string) bool {
	return true
}
