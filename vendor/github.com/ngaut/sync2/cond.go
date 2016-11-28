// Copyright 2013, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync2

import (
	"sync"
)

// Cond is an alternate implementation of sync.Cond
type Cond struct {
	L       sync.Locker
	sema    chan struct{}
	waiters AtomicInt64
}

func NewCond(l sync.Locker) *Cond {
	return &Cond{L: l, sema: make(chan struct{})}
}

func (c *Cond) Wait() {
	c.waiters.Add(1)
	c.L.Unlock()
	<-c.sema
	c.L.Lock()
}

func (c *Cond) Signal() {
	for {
		w := c.waiters.Get()
		if w == 0 {
			return
		}
		if c.waiters.CompareAndSwap(w, w-1) {
			break
		}
	}
	c.sema <- struct{}{}
}

func (c *Cond) Broadcast() {
	var w int64
	for {
		w = c.waiters.Get()
		if w == 0 {
			return
		}
		if c.waiters.CompareAndSwap(w, 0) {
			break
		}
	}
	for i := int64(0); i < w; i++ {
		c.sema <- struct{}{}
	}
}
