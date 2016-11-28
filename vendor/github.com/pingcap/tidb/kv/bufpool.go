// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package kv

import (
	"sync"

	"github.com/ngaut/log"
)

// A cache holds a set of reusable objects.
// The slice is a stack (LIFO).
// If more are needed, the cache creates them by calling new.
type cache struct {
	mu    sync.Mutex
	name  string
	saved []MemBuffer
	// factory
	fact func() MemBuffer
}

func (c *cache) put(x MemBuffer) {
	c.mu.Lock()
	if len(c.saved) < cap(c.saved) {
		c.saved = append(c.saved, x)
	} else {
		log.Warnf("%s is full, size: %d, you may need to increase pool size", c.name, len(c.saved))
	}
	c.mu.Unlock()
}

func (c *cache) get() MemBuffer {
	c.mu.Lock()
	n := len(c.saved)
	if n == 0 {
		c.mu.Unlock()
		return c.fact()
	}
	x := c.saved[n-1]
	c.saved = c.saved[0 : n-1]
	c.mu.Unlock()
	return x
}

func newCache(name string, cap int, fact func() MemBuffer) *cache {
	return &cache{name: name, saved: make([]MemBuffer, 0, cap), fact: fact}
}
