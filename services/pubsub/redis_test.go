// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRedisBroker(t *testing.T) *RedisBroker {
	t.Helper()
	mr := miniredis.RunT(t)
	b, err := NewRedisBroker("redis://" + mr.Addr() + "/0")
	require.NoError(t, err)
	return b
}

func TestRedisBroker_SubscribePublish(t *testing.T) {
	b := newTestRedisBroker(t)
	ch, cancel := b.Subscribe("topic")
	defer cancel()

	b.Publish("topic", []byte("hello"))

	select {
	case msg := <-ch:
		assert.Equal(t, []byte("hello"), msg)
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive published message")
	}
}

func TestRedisBroker_TopicIsolation(t *testing.T) {
	b := newTestRedisBroker(t)
	chA, cancelA := b.Subscribe("a")
	defer cancelA()
	chB, cancelB := b.Subscribe("b")
	defer cancelB()

	b.Publish("a", []byte("only-a"))

	select {
	case msg := <-chA:
		assert.Equal(t, []byte("only-a"), msg)
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive message on topic a")
	}
	select {
	case msg := <-chB:
		t.Fatalf("topic b unexpectedly got message: %s", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestRedisBroker_FanOutToLocalSubscribers(t *testing.T) {
	b := newTestRedisBroker(t)

	const n = 3
	channels := make([]<-chan []byte, n)
	cancels := make([]func(), n)
	for i := range n {
		channels[i], cancels[i] = b.Subscribe("topic")
	}
	defer func() {
		for _, c := range cancels {
			c()
		}
	}()

	b.Publish("topic", []byte("broadcast"))

	for i, ch := range channels {
		select {
		case msg := <-ch:
			assert.Equal(t, []byte("broadcast"), msg, "subscriber %d", i)
		case <-time.After(2 * time.Second):
			t.Fatalf("subscriber %d did not receive message", i)
		}
	}
}

func TestRedisBroker_CancelClosesChannelAndCleansTopic(t *testing.T) {
	b := newTestRedisBroker(t)
	ch, cancel := b.Subscribe("topic")
	cancel()

	_, ok := <-ch
	assert.False(t, ok, "channel must be closed after cancel")

	b.mu.RLock()
	_, present := b.topics["topic"]
	b.mu.RUnlock()
	assert.False(t, present, "topic state must be removed after last subscriber cancels")
}

func TestRedisBroker_HasTopicSubscribersAlwaysTrue(t *testing.T) {
	b := newTestRedisBroker(t)
	// Conservative answer so publishers don't skip cross-process subscribers.
	assert.True(t, b.HasTopicSubscribers("anything"))
}

// Two RedisBroker instances sharing one Redis simulate two Gitea processes:
// a publish on one must reach a subscriber on the other.
func TestRedisBroker_CrossBroker(t *testing.T) {
	mr := miniredis.RunT(t)
	conn := "redis://" + mr.Addr() + "/0"

	publisher, err := NewRedisBroker(conn)
	require.NoError(t, err)
	subscriber, err := NewRedisBroker(conn)
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
