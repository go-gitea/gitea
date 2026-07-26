// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import (
	"testing"
	"time"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRedisBroker runs the shared broker scenarios against a real Redis-backed
// RedisBroker. Delivery crosses a Redis round-trip (hence the wider timeout) and
// HasTopicSubscribers answers conservatively true, so subscriber tracking is not
// exact.
func TestRedisBroker(t *testing.T) {
	redisConn, redisCancel := test.PrepareTestRedis(t)
	defer redisCancel()
	testBrokerBasic(t, func(t *testing.T) Broker {
		broker, err := NewRedisBroker(redisConn)
		require.NoError(t, err)
		return broker
	}, 2*time.Second, false)
}

// Backend-specific: RedisBroker tears down its per-topic Redis subscription and
// internal state once the last local subscriber cancels.
func TestRedisBroker_CancelCleansTopicState(t *testing.T) {
	redisConn, redisCancel := test.PrepareTestRedis(t)
	defer redisCancel()
	b, err := NewRedisBroker(redisConn)
	require.NoError(t, err)
	ch, cancel := b.Subscribe("topic")
	cancel()

	_, ok := <-ch
	assert.False(t, ok, "channel must be closed after cancel")

	b.mu.RLock()
	_, present := b.topics["topic"]
	b.mu.RUnlock()
	assert.False(t, present, "topic state must be removed after last subscriber cancels")
}

// Backend-specific: two RedisBroker instances sharing one Redis simulate two
// Gitea processes - a publish on one must reach a subscriber on the other.
func TestRedisBroker_CrossBroker(t *testing.T) {
	redisConn, redisCancel := test.PrepareTestRedis(t)
	defer redisCancel()

	publisher, err := NewRedisBroker(redisConn)
	require.NoError(t, err)
	subscriber, err := NewRedisBroker(redisConn)
	require.NoError(t, err)

	ch, cancel := subscriber.Subscribe("topic")
	defer cancel()

	publisher.Publish("topic", []byte("cross-process"))

	select {
	case msg := <-ch:
		assert.Equal(t, []byte("cross-process"), msg)
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber on second broker did not receive message")
	}
}
