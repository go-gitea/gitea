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

// TestMemoryBroker runs the shared broker scenarios against MemoryBroker.
// Memory delivers synchronously and tracks live subscribers exactly.
func TestMemoryBroker(t *testing.T) {
	testBrokerBasic(t, func(t *testing.T) Broker {
		return NewMemoryBroker()
	}, time.Second, true)
}

// Backend-specific: MemoryBroker prunes empty topics from its internal map so
// idle-user entries don't accumulate.
func TestMemoryBroker_CancelDeletesEmptyTopic(t *testing.T) {
	b := NewMemoryBroker()
	_, cancel := b.Subscribe("topic")
	cancel()
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, present := b.subs["topic"]
	assert.False(t, present, "empty topic must be removed from the map so idle-user entries don't accumulate")
}

// Backend-specific: stresses MemoryBroker's cancel/Publish mutex interlock that
// prevents send-on-closed-channel panics.
func TestMemoryBroker_ConcurrentPublishSubscribeCancel(t *testing.T) {
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
