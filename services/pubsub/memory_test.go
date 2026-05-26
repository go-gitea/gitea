// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBroker_PublishWithoutSubscribers(t *testing.T) {
	b := NewMemoryBroker()
	// Must not block or panic when no subscribers exist.
	b.Publish("nobody", []byte("msg"))
}

func TestBroker_SubscribeReceivesPublishedMessages(t *testing.T) {
	b := NewMemoryBroker()
	ch, cancel := b.Subscribe("topic")
	defer cancel()

	b.Publish("topic", []byte("hello"))

	select {
	case msg := <-ch:
		assert.Equal(t, []byte("hello"), msg)
	case <-time.After(time.Second):
		t.Fatal("did not receive published message")
	}
}

func TestBroker_FanOutToAllSubscribers(t *testing.T) {
	b := NewMemoryBroker()
	const n = 5
	channels := make([]<-chan []byte, n)
	cancels := make([]func(), n)
	for idx := range n {
		channels[idx], cancels[idx] = b.Subscribe("topic")
	}
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
	}()

	b.Publish("topic", []byte("broadcast"))

	for idx, ch := range channels {
		select {
		case msg := <-ch:
			assert.Equal(t, []byte("broadcast"), msg, "subscriber %d", idx)
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d did not receive message", idx)
		}
	}
}

func TestBroker_TopicIsolation(t *testing.T) {
	b := NewMemoryBroker()
	chA, cancelA := b.Subscribe("a")
	defer cancelA()
	chB, cancelB := b.Subscribe("b")
	defer cancelB()

	b.Publish("a", []byte("only-a"))

	select {
	case msg := <-chA:
		assert.Equal(t, []byte("only-a"), msg)
	case <-time.After(time.Second):
		t.Fatal("did not receive message on topic a")
	}
	select {
	case msg := <-chB:
		t.Fatalf("topic b unexpectedly got message: %s", msg)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestBroker_CancelStopsDelivery(t *testing.T) {
	b := NewMemoryBroker()
	ch, cancel := b.Subscribe("topic")

	cancel()

	// Channel should be closed.
	_, ok := <-ch
	assert.False(t, ok, "channel must be closed after cancel")

	// Publishing after cancel must not panic or block.
	b.Publish("topic", []byte("after-cancel"))

	assert.False(t, b.HasTopicSubscribers("topic"))
}

func TestBroker_SlowSubscriberDoesNotBlockOthers(t *testing.T) {
	b := NewMemoryBroker()
	_, cancelSlow := b.Subscribe("topic")
	defer cancelSlow()
	fast, cancelFast := b.Subscribe("topic")
	defer cancelFast()

	// Fill the slow subscriber's buffer (capacity 8) — it never drains.
	for idx := range 8 {
		b.Publish("topic", []byte{byte(idx)})
	}
	// Drain fast so we can measure whether the next publish reaches it.
	for range 8 {
		<-fast
	}

	// This publish would block on `slow` if the broker used blocking sends.
	done := make(chan struct{})
	go func() {
		b.Publish("topic", []byte("final"))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on slow subscriber")
	}

	select {
	case msg := <-fast:
		assert.Equal(t, []byte("final"), msg)
	case <-time.After(time.Second):
		t.Fatal("fast subscriber did not receive message after slow subscriber stalled")
	}
}

func TestBroker_CancelDeletesEmptyTopic(t *testing.T) {
	b := NewMemoryBroker()
	_, cancel := b.Subscribe("topic")
	cancel()
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, present := b.subs["topic"]
	assert.False(t, present, "empty topic must be removed from the map so idle-user entries don't accumulate")
}

func TestBroker_ConcurrentPublishSubscribeCancel(t *testing.T) {
	b := NewMemoryBroker()

	const writers = 4
	const readers = 8
	const duration = 200 * time.Millisecond

	stop := make(chan struct{})
	var wg sync.WaitGroup

	var published atomic.Int64
	for range writers {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					b.Publish("topic", []byte("x"))
					published.Add(1)
				}
			}
		})
	}

	for range readers {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					ch, cancel := b.Subscribe("topic")
					// Drain a few messages then cancel — this exercises the
					// cancel/Publish interlock that prevents send-on-closed.
					for range 3 {
						select {
						case <-ch:
						case <-time.After(10 * time.Millisecond):
						}
					}
					cancel()
				}
			}
		})
	}

	time.Sleep(duration)
	close(stop)
	wg.Wait()

	// Test passes if no panic (send on closed channel) and no deadlock.
	assert.Positive(t, published.Load())
}

func TestUserTopic(t *testing.T) {
	assert.Equal(t, "user-42", UserTopic(42))
	assert.Equal(t, "user-0", UserTopic(0))
}
