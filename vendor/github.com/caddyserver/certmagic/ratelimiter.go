// Copyright 2015 Matthew Holt
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package certmagic

import (
	"context"
	"log"
	"runtime"
	"sync"
	"time"
)

// NewRateLimiter returns a rate limiter that allows up to maxEvents
// in a sliding window of size window. If maxEvents and window are
// both 0, or if maxEvents is non-zero and window is 0, rate limiting
// is disabled. This function panics if maxEvents is less than 0 or
// if maxEvents is 0 and window is non-zero, which is considered to be
// an invalid configuration, as it would never allow events.
func NewRateLimiter(maxEvents int, window time.Duration) *RingBufferRateLimiter {
	if maxEvents < 0 {
		panic("maxEvents cannot be less than zero")
	}
	if maxEvents == 0 && window != 0 {
		panic("invalid configuration: maxEvents = 0 and window != 0 would not allow any events")
	}
	rbrl := &RingBufferRateLimiter{
		window:  window,
		ring:    make([]time.Time, maxEvents),
		started: make(chan struct{}),
		stopped: make(chan struct{}),
		ticket:  make(chan struct{}),
	}
	go rbrl.loop()
	<-rbrl.started // make sure loop is ready to receive before we return
	return rbrl
}

// RingBufferRateLimiter uses a ring to enforce rate limits
// consisting of a maximum number of events within a single
// sliding window of a given duration. An empty value is
// not valid; use NewRateLimiter to get one.
type RingBufferRateLimiter struct {
	window  time.Duration
	ring    []time.Time // maxEvents == len(ring)
	cursor  int         // always points to the oldest timestamp
	mu      sync.Mutex  // protects ring, cursor, and window
	started chan struct{}
	stopped chan struct{}
	ticket  chan struct{}
}

// Stop cleans up r's scheduling goroutine.
func (r *RingBufferRateLimiter) Stop() {
	close(r.stopped)
}

func (r *RingBufferRateLimiter) loop() {
	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, stackTraceBufferSize)
			buf = buf[:runtime.Stack(buf, false)]
			log.Printf("panic: ring buffer rate limiter: %v\n%s", err, buf)
		}
	}()

	for {
		// if we've been stopped, return
		select {
		case <-r.stopped:
			return
		default:
		}

		if len(r.ring) == 0 {
			if r.window == 0 {
				// rate limiting is disabled; always allow immediately
				r.permit()
				continue
			}
			panic("invalid configuration: maxEvents = 0 and window != 0 does not allow any events")
		}

		// wait until next slot is available or until we've been stopped
		r.mu.Lock()
		then := r.ring[r.cursor].Add(r.window)
		r.mu.Unlock()
		waitDuration := time.Until(then)
		waitTimer := time.NewTimer(waitDuration)
		select {
		case <-waitTimer.C:
			r.permit()
		case <-r.stopped:
			waitTimer.Stop()
			return
		}
	}
}

// Allow returns true if the event is allowed to
// happen right now. It does not wait. If the event
// is allowed, a ticket is claimed.
func (r *RingBufferRateLimiter) Allow() bool {
	select {
	case <-r.ticket:
		return true
	default:
		return false
	}
}

// Wait blocks until the event is allowed to occur. It returns an
// error if the context is cancelled.
func (r *RingBufferRateLimiter) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return context.Canceled
	case <-r.ticket:
		return nil
	}
}

// MaxEvents returns the maximum number of events that
// are allowed within the sliding window.
func (r *RingBufferRateLimiter) MaxEvents() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.ring)
}

// SetMaxEvents changes the maximum number of events that are
// allowed in the sliding window. If the new limit is lower,
// the oldest events will be forgotten. If the new limit is
// higher, the window will suddenly have capacity for new
// reservations. It panics if maxEvents is 0 and window size
// is not zero.
func (r *RingBufferRateLimiter) SetMaxEvents(maxEvents int) {
	newRing := make([]time.Time, maxEvents)
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.window != 0 && maxEvents == 0 {
		panic("invalid configuration: maxEvents = 0 and window != 0 would not allow any events")
	}

	// only make the change if the new limit is different
	if maxEvents == len(r.ring) {
		return
	}

	// the new ring may be smaller; fast-forward to the
	// oldest timestamp that will be kept in the new
	// ring so the oldest ones are forgotten and the
	// newest ones will be remembered
	sizeDiff := len(r.ring) - maxEvents
	for i := 0; i < sizeDiff; i++ {
		r.advance()
	}

	if len(r.ring) > 0 {
		// copy timestamps into the new ring until we
		// have either copied all of them or have reached
		// the capacity of the new ring
		startCursor := r.cursor
		for i := 0; i < len(newRing); i++ {
			newRing[i] = r.ring[r.cursor]
			r.advance()
			if r.cursor == startCursor {
				// new ring is larger than old one;
				// "we've come full circle"
				break
			}
		}
	}

	r.ring = newRing
	r.cursor = 0
}

// Window returns the size of the sliding window.
func (r *RingBufferRateLimiter) Window() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.window
}

// SetWindow changes r's sliding window duration to window.
// Goroutines that are already blocked on a call to Wait()
// will not be affected. It panics if window is non-zero
// but the max event limit is 0.
func (r *RingBufferRateLimiter) SetWindow(window time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if window != 0 && len(r.ring) == 0 {
		panic("invalid configuration: maxEvents = 0 and window != 0 would not allow any events")
	}
	r.window = window
}

// permit allows one event through the throttle. This method
// blocks until a goroutine is waiting for a ticket or until
// the rate limiter is stopped.
func (r *RingBufferRateLimiter) permit() {
	for {
		select {
		case r.started <- struct{}{}:
			// notify parent goroutine that we've started; should
			// only happen once, before constructor returns
			continue
		case <-r.stopped:
			return
		case r.ticket <- struct{}{}:
			r.mu.Lock()
			defer r.mu.Unlock()
			if len(r.ring) > 0 {
				r.ring[r.cursor] = time.Now()
				r.advance()
			}
			return
		}
	}
}

// advance moves the cursor to the next position.
// It is NOT safe for concurrent use, so it must
// be called inside a lock on r.mu.
func (r *RingBufferRateLimiter) advance() {
	r.cursor++
	if r.cursor >= len(r.ring) {
		r.cursor = 0
	}
}
