// Copyright 2012, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pools

import (
	"fmt"
	"sync"
	"time"
)

// RoundRobin is deprecated. Use ResourcePool instead.
// RoundRobin allows you to use a pool of resources in a round robin fashion.
type RoundRobin struct {
	mu          sync.Mutex
	available   *sync.Cond
	resources   chan fifoWrapper
	size        int64
	factory     Factory
	idleTimeout time.Duration

	// stats
	waitCount int64
	waitTime  time.Duration
}

type fifoWrapper struct {
	resource Resource
	timeUsed time.Time
}

// NewRoundRobin creates a new RoundRobin pool.
// capacity is the maximum number of resources RoundRobin will create.
// factory will be the function used to create resources.
// If a resource is unused beyond idleTimeout, it's discarded.
func NewRoundRobin(capacity int, idleTimeout time.Duration) *RoundRobin {
	r := &RoundRobin{
		resources:   make(chan fifoWrapper, capacity),
		size:        0,
		idleTimeout: idleTimeout,
	}
	r.available = sync.NewCond(&r.mu)
	return r
}

// Open starts allowing the creation of resources
func (rr *RoundRobin) Open(factory Factory) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.factory = factory
}

// Close empties the pool calling Close on all its resources.
// It waits for all resources to be returned (Put).
func (rr *RoundRobin) Close() {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	for rr.size > 0 {
		select {
		case fw := <-rr.resources:
			go fw.resource.Close()
			rr.size--
		default:
			rr.available.Wait()
		}
	}
	rr.factory = nil
}

func (rr *RoundRobin) IsClosed() bool {
	return rr.factory == nil
}

// Get will return the next available resource. If none is available, and capacity
// has not been reached, it will create a new one using the factory. Otherwise,
// it will indefinitely wait till the next resource becomes available.
func (rr *RoundRobin) Get() (resource Resource, err error) {
	return rr.get(true)
}

// TryGet will return the next available resource. If none is available, and capacity
// has not been reached, it will create a new one using the factory. Otherwise,
// it will return nil with no error.
func (rr *RoundRobin) TryGet() (resource Resource, err error) {
	return rr.get(false)
}

func (rr *RoundRobin) get(wait bool) (resource Resource, err error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	// Any waits in this loop will release the lock, and it will be
	// reacquired before the waits return.
	for {
		select {
		case fw := <-rr.resources:
			// Found a free resource in the channel
			if rr.idleTimeout > 0 && fw.timeUsed.Add(rr.idleTimeout).Sub(time.Now()) < 0 {
				// resource has been idle for too long. Discard & go for next.
				go fw.resource.Close()
				rr.size--
				// Nobody else should be waiting, but signal anyway.
				rr.available.Signal()
				continue
			}
			return fw.resource, nil
		default:
			// resource channel is empty
			if rr.size >= int64(cap(rr.resources)) {
				// The pool is full
				if wait {
					start := time.Now()
					rr.available.Wait()
					rr.recordWait(start)
					continue
				}
				return nil, nil
			}
			// Pool is not full. Create a resource.
			if resource, err = rr.waitForCreate(); err != nil {
				// size was decremented, and somebody could be waiting.
				rr.available.Signal()
				return nil, err
			}
			// Creation successful. Account for this by incrementing size.
			rr.size++
			return resource, err
		}
	}
}

func (rr *RoundRobin) recordWait(start time.Time) {
	rr.waitCount++
	rr.waitTime += time.Now().Sub(start)
}

func (rr *RoundRobin) waitForCreate() (resource Resource, err error) {
	// Prevent thundering herd: increment size before creating resource, and decrement after.
	rr.size++
	rr.mu.Unlock()
	defer func() {
		rr.mu.Lock()
		rr.size--
	}()
	return rr.factory()
}

// Put will return a resource to the pool. You MUST return every resource to the pool,
// even if it's closed. If a resource is closed, you should call Put(nil).
func (rr *RoundRobin) Put(resource Resource) {
	rr.mu.Lock()
	defer rr.available.Signal()
	defer rr.mu.Unlock()

	if rr.size > int64(cap(rr.resources)) {
		if resource != nil {
			go resource.Close()
		}
		rr.size--
	} else if resource == nil {
		rr.size--
	} else {
		if len(rr.resources) == cap(rr.resources) {
			panic("unexpected")
		}
		rr.resources <- fifoWrapper{resource, time.Now()}
	}
}

// Set capacity changes the capacity of the pool.
// You can use it to expand or shrink.
func (rr *RoundRobin) SetCapacity(capacity int) error {
	rr.mu.Lock()
	defer rr.available.Broadcast()
	defer rr.mu.Unlock()

	nr := make(chan fifoWrapper, capacity)
	// This loop transfers resources from the old channel
	// to the new one, until it fills up or runs out.
	// It discards extras, if any.
	for {
		select {
		case fw := <-rr.resources:
			if len(nr) < cap(nr) {
				nr <- fw
			} else {
				go fw.resource.Close()
				rr.size--
			}
			continue
		default:
		}
		break
	}
	rr.resources = nr
	return nil
}

func (rr *RoundRobin) SetIdleTimeout(idleTimeout time.Duration) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.idleTimeout = idleTimeout
}

func (rr *RoundRobin) StatsJSON() string {
	s, c, a, wc, wt, it := rr.Stats()
	return fmt.Sprintf("{\"Size\": %v, \"Capacity\": %v, \"Available\": %v, \"WaitCount\": %v, \"WaitTime\": %v, \"IdleTimeout\": %v}", s, c, a, wc, int64(wt), int64(it))
}

func (rr *RoundRobin) Stats() (size, capacity, available, waitCount int64, waitTime, idleTimeout time.Duration) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.size, int64(cap(rr.resources)), int64(len(rr.resources)), rr.waitCount, rr.waitTime, rr.idleTimeout
}
