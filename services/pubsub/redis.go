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
	if err := client.Ping(graceful.GetManager().ShutdownContext()).Err(); err != nil {
		return nil, err
	}
	return &RedisBroker{
		client: client,
		topics: make(map[string]*redisTopic),
	}, nil
}

func (b *RedisBroker) Subscribe(topic string) (<-chan []byte, func()) {
	sub := &redisSub{ch: make(chan []byte, 8)}

	b.mu.Lock()
	state, exists := b.topics[topic]
	if !exists {
		// graceful.ShutdownContext so the reader loop dies cleanly on Gitea shutdown
		// even if every local subscriber has already cancelled.
		ctx, cancel := context.WithCancel(graceful.GetManager().ShutdownContext())
		ps := b.client.Subscribe(ctx)
		subscribeCtx, subscribeCancel := context.WithTimeout(ctx, redisSubscribeTimeout)
		err := ps.Subscribe(subscribeCtx, topic)
		subscribeCancel()
		if err != nil {
			b.mu.Unlock()
			cancel()
			_ = ps.Close()
			close(sub.ch)
			log.Error("pubsub redis: subscribe %q: %v", topic, err)
			return sub.ch, func() {}
		}
		state = &redisTopic{ps: ps, cancel: cancel}
		b.topics[topic] = state
		go b.readLoop(ctx, topic, ps)
	}
	state.subs = append(state.subs, sub)
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
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
	return sub.ch, cancel
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
