// Copyright 2014, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pools

import (
	"fmt"
	"sync"
)

// IDPool is used to ensure that the set of IDs in use concurrently never
// contains any duplicates. The IDs start at 1 and increase without bound, but
// will never be larger than the peak number of concurrent uses.
//
// IDPool's Get() and Set() methods can be used concurrently.
type IDPool struct {
	sync.Mutex

	// used holds the set of values that have been returned to us with Put().
	used map[uint32]bool
	// maxUsed remembers the largest value we've given out.
	maxUsed uint32
}

// NewIDPool creates and initializes an IDPool.
func NewIDPool() *IDPool {
	return &IDPool{
		used: make(map[uint32]bool),
	}
}

// Get returns an ID that is unique among currently active users of this pool.
func (pool *IDPool) Get() (id uint32) {
	pool.Lock()
	defer pool.Unlock()

	// Pick a value that's been returned, if any.
	for key, _ := range pool.used {
		delete(pool.used, key)
		return key
	}

	// No recycled IDs are available, so increase the pool size.
	pool.maxUsed += 1
	return pool.maxUsed
}

// Put recycles an ID back into the pool for others to use. Putting back a value
// or 0, or a value that is not currently "checked out", will result in a panic
// because that should never happen except in the case of a programming error.
func (pool *IDPool) Put(id uint32) {
	pool.Lock()
	defer pool.Unlock()

	if id < 1 || id > pool.maxUsed {
		panic(fmt.Errorf("IDPool.Put(%v): invalid value, must be in the range [1,%v]", id, pool.maxUsed))
	}

	if pool.used[id] {
		panic(fmt.Errorf("IDPool.Put(%v): can't put value that was already recycled", id))
	}

	// If we're recycling maxUsed, just shrink the pool.
	if id == pool.maxUsed {
		pool.maxUsed = id - 1
		return
	}

	// Add it to the set of recycled IDs.
	pool.used[id] = true
}
