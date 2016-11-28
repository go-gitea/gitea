// Copyright 2012, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pools provides functionality to manage and reuse resources
// like connections.
package pools

import (
	"fmt"
	"time"

	"github.com/ngaut/sync2"
)

var (
	CLOSED_ERR = fmt.Errorf("ResourcePool is closed")
)

// Factory is a function that can be used to create a resource.
type Factory func() (Resource, error)

// Every resource needs to suport the Resource interface.
// Thread synchronization between Close() and IsClosed()
// is the responsibility the caller.
type Resource interface {
	Close()
}

// ResourcePool allows you to use a pool of resources.
type ResourcePool struct {
	resources   chan resourceWrapper
	factory     Factory
	capacity    sync2.AtomicInt64
	idleTimeout sync2.AtomicDuration

	// stats
	waitCount sync2.AtomicInt64
	waitTime  sync2.AtomicDuration
}

type resourceWrapper struct {
	resource Resource
	timeUsed time.Time
}

// NewResourcePool creates a new ResourcePool pool.
// capacity is the initial capacity of the pool.
// maxCap is the maximum capacity.
// If a resource is unused beyond idleTimeout, it's discarded.
// An idleTimeout of 0 means that there is no timeout.
func NewResourcePool(factory Factory, capacity, maxCap int, idleTimeout time.Duration) *ResourcePool {
	if capacity <= 0 || maxCap <= 0 || capacity > maxCap {
		panic(fmt.Errorf("Invalid/out of range capacity"))
	}
	rp := &ResourcePool{
		resources:   make(chan resourceWrapper, maxCap),
		factory:     factory,
		capacity:    sync2.AtomicInt64(capacity),
		idleTimeout: sync2.AtomicDuration(idleTimeout),
	}
	for i := 0; i < capacity; i++ {
		rp.resources <- resourceWrapper{}
	}
	return rp
}

// Close empties the pool calling Close on all its resources.
// You can call Close while there are outstanding resources.
// It waits for all resources to be returned (Put).
// After a Close, Get and TryGet are not allowed.
func (rp *ResourcePool) Close() {
	rp.SetCapacity(0)
}

func (rp *ResourcePool) IsClosed() (closed bool) {
	return rp.capacity.Get() == 0
}

// Get will return the next available resource. If capacity
// has not been reached, it will create a new one using the factory. Otherwise,
// it will indefinitely wait till the next resource becomes available.
func (rp *ResourcePool) Get() (resource Resource, err error) {
	return rp.get(true)
}

// TryGet will return the next available resource. If none is available, and capacity
// has not been reached, it will create a new one using the factory. Otherwise,
// it will return nil with no error.
func (rp *ResourcePool) TryGet() (resource Resource, err error) {
	return rp.get(false)
}

func (rp *ResourcePool) get(wait bool) (resource Resource, err error) {
	// Fetch
	var wrapper resourceWrapper
	var ok bool
	select {
	case wrapper, ok = <-rp.resources:
	default:
		if !wait {
			return nil, nil
		}
		startTime := time.Now()
		wrapper, ok = <-rp.resources
		rp.recordWait(startTime)
	}
	if !ok {
		return nil, CLOSED_ERR
	}

	// Unwrap
	timeout := rp.idleTimeout.Get()
	if wrapper.resource != nil && timeout > 0 && wrapper.timeUsed.Add(timeout).Sub(time.Now()) < 0 {
		wrapper.resource.Close()
		wrapper.resource = nil
	}
	if wrapper.resource == nil {
		wrapper.resource, err = rp.factory()
		if err != nil {
			rp.resources <- resourceWrapper{}
		}
	}
	return wrapper.resource, err
}

// Put will return a resource to the pool. For every successful Get,
// a corresponding Put is required. If you no longer need a resource,
// you will need to call Put(nil) instead of returning the closed resource.
// The will eventually cause a new resource to be created in its place.
func (rp *ResourcePool) Put(resource Resource) {
	var wrapper resourceWrapper
	if resource != nil {
		wrapper = resourceWrapper{resource, time.Now()}
	}
	select {
	case rp.resources <- wrapper:
	default:
		panic(fmt.Errorf("Attempt to Put into a full ResourcePool"))
	}
}

// SetCapacity changes the capacity of the pool.
// You can use it to shrink or expand, but not beyond
// the max capacity. If the change requires the pool
// to be shrunk, SetCapacity waits till the necessary
// number of resources are returned to the pool.
// A SetCapacity of 0 is equivalent to closing the ResourcePool.
func (rp *ResourcePool) SetCapacity(capacity int) error {
	if capacity < 0 || capacity > cap(rp.resources) {
		return fmt.Errorf("capacity %d is out of range", capacity)
	}

	// Atomically swap new capacity with old, but only
	// if old capacity is non-zero.
	var oldcap int
	for {
		oldcap = int(rp.capacity.Get())
		if oldcap == 0 {
			return CLOSED_ERR
		}
		if oldcap == capacity {
			return nil
		}
		if rp.capacity.CompareAndSwap(int64(oldcap), int64(capacity)) {
			break
		}
	}

	if capacity < oldcap {
		for i := 0; i < oldcap-capacity; i++ {
			wrapper := <-rp.resources
			if wrapper.resource != nil {
				wrapper.resource.Close()
			}
		}
	} else {
		for i := 0; i < capacity-oldcap; i++ {
			rp.resources <- resourceWrapper{}
		}
	}
	if capacity == 0 {
		close(rp.resources)
	}
	return nil
}

func (rp *ResourcePool) recordWait(start time.Time) {
	rp.waitCount.Add(1)
	rp.waitTime.Add(time.Now().Sub(start))
}

func (rp *ResourcePool) SetIdleTimeout(idleTimeout time.Duration) {
	rp.idleTimeout.Set(idleTimeout)
}

func (rp *ResourcePool) StatsJSON() string {
	c, a, mx, wc, wt, it := rp.Stats()
	return fmt.Sprintf(`{"Capacity": %v, "Available": %v, "MaxCapacity": %v, "WaitCount": %v, "WaitTime": %v, "IdleTimeout": %v}`, c, a, mx, wc, int64(wt), int64(it))
}

func (rp *ResourcePool) Stats() (capacity, available, maxCap, waitCount int64, waitTime, idleTimeout time.Duration) {
	return rp.Capacity(), rp.Available(), rp.MaxCap(), rp.WaitCount(), rp.WaitTime(), rp.IdleTimeout()
}

func (rp *ResourcePool) Capacity() int64 {
	return rp.capacity.Get()
}

func (rp *ResourcePool) Available() int64 {
	return int64(len(rp.resources))
}

func (rp *ResourcePool) MaxCap() int64 {
	return int64(cap(rp.resources))
}

func (rp *ResourcePool) WaitCount() int64 {
	return rp.waitCount.Get()
}

func (rp *ResourcePool) WaitTime() time.Duration {
	return rp.waitTime.Get()
}

func (rp *ResourcePool) IdleTimeout() time.Duration {
	return rp.idleTimeout.Get()
}
