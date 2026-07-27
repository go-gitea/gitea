// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newBrokerFunc builds a fresh Broker bound to the test's lifecycle.
type newBrokerFunc func(t *testing.T) Broker

// testBrokerBasic runs the behavior every Broker backend must share. Each backend
// invokes it from its own *_test.go (like testQueueBasic in modules/queue) so
// memory and redis prove identical semantics against the same scenarios.
//
// recvTimeout absorbs redis's network round-trip (memory delivers synchronously).
func testBrokerBasic(t *testing.T, newBroker newBrokerFunc, recvTimeout time.Duration) {
	t.Run("PublishWithoutSubscribers", func(t *testing.T) {
		b := newBroker(t)
		b.Publish("nobody", []byte("msg")) // must not block or panic
	})

	t.Run("SubscribeReceivesPublished", func(t *testing.T) {
		b := newBroker(t)
		ch, cancel := b.Subscribe("topic")
		defer cancel()

		b.Publish("topic", []byte("hello"))
		assert.Equal(t, []byte("hello"), recvWithin(t, ch, recvTimeout))
	})

	t.Run("FanOutToAllSubscribers", func(t *testing.T) {
		b := newBroker(t)
		const n = 3
		channels := make([]<-chan []byte, n)
		for i := range n {
			ch, cancel := b.Subscribe("topic")
			defer cancel()
			channels[i] = ch
		}

		b.Publish("topic", []byte("broadcast"))
		for i, ch := range channels {
			assert.Equal(t, []byte("broadcast"), recvWithin(t, ch, recvTimeout), "subscriber %d", i)
		}
	})

	t.Run("TopicIsolation", func(t *testing.T) {
		b := newBroker(t)
		chA, cancelA := b.Subscribe("a")
		defer cancelA()
		chB, cancelB := b.Subscribe("b")
		defer cancelB()

		b.Publish("a", []byte("only-a"))
		assert.Equal(t, []byte("only-a"), recvWithin(t, chA, recvTimeout))
		assertQuiet(t, chB, 100*time.Millisecond) // topic b must stay silent
	})

	t.Run("CancelStopsDelivery", func(t *testing.T) {
		b := newBroker(t)
		ch, cancel := b.Subscribe("topic")

		cancel()
		_, ok := <-ch
		assert.False(t, ok, "channel must be closed after cancel")

		b.Publish("topic", []byte("after-cancel")) // must not panic or block
	})

	t.Run("CancelIsIdempotent", func(t *testing.T) {
		b := newBroker(t)
		_, cancel := b.Subscribe("topic")
		cancel()
		assert.NotPanics(t, cancel, "cancel must be safe to call more than once")
	})

	// What each backend reports with no live subscriber differs, so that stays in
	// the backend's own test file.
	t.Run("HasTopicSubscribers", func(t *testing.T) {
		b := newBroker(t)
		_, cancel := b.Subscribe("topic")
		defer cancel()
		assert.True(t, b.HasTopicSubscribers("topic"), "must report subscribers while one is live")
	})

	t.Run("SlowSubscriberDropsWithoutBlocking", func(t *testing.T) {
		b := newBroker(t)
		_, cancelSlow := b.Subscribe("topic") // never drained, buffer overflows
		defer cancelSlow()
		fast, cancelFast := b.Subscribe("topic")
		defer cancelFast()

		// Drain fast concurrently so it keeps up while slow's buffer fills.
		got := make(chan struct{}, 1)
		done := make(chan struct{})
		defer close(done)
		go func() {
			for {
				select {
				case _, ok := <-fast:
					if !ok {
						return
					}
					select {
					case got <- struct{}{}:
					default:
					}
				case <-done:
					return
				}
			}
		}()

		// Far more than the 8-slot buffer: Publish must never block on slow.
		const n = 50
		published := make(chan struct{})
		go func() {
			for i := range n {
				b.Publish("topic", []byte{byte(i)})
			}
			close(published)
		}()
		select {
		case <-published:
		case <-time.After(recvTimeout):
			t.Fatal("Publish blocked on slow subscriber")
		}

		// The fast subscriber still receives while slow is stuck.
		select {
		case <-got:
		case <-time.After(recvTimeout):
			t.Fatal("fast subscriber received nothing while slow subscriber stalled")
		}
	})
}

// recvWithin returns the next message or fails if none arrives before timeout.
func recvWithin(t *testing.T, ch <-chan []byte, timeout time.Duration) []byte {
	t.Helper()
	select {
	case msg, ok := <-ch:
		require.True(t, ok, "channel closed before a message arrived")
		return msg
	case <-time.After(timeout):
		t.Fatalf("timed out after %s waiting for a message", timeout)
		return nil
	}
}

// assertQuiet fails if any message arrives on ch within d.
func assertQuiet(t *testing.T, ch <-chan []byte, d time.Duration) {
	t.Helper()
	select {
	case msg := <-ch:
		t.Fatalf("unexpected message: %s", msg)
	case <-time.After(d):
	}
}

func TestUserTopic(t *testing.T) {
	assert.Equal(t, "user-42", UserTopic(42))
	assert.Equal(t, "user-0", UserTopic(0))
}
