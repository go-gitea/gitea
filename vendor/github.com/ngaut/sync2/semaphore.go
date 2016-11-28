// Copyright 2012, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync2

// What's in a name? Channels have all you need to emulate a counting
// semaphore with a boatload of extra functionality. However, in some
// cases, you just want a familiar API.

import (
	"time"
)

// Semaphore is a counting semaphore with the option to
// specify a timeout.
type Semaphore struct {
	slots   chan struct{}
	timeout time.Duration
}

// NewSemaphore creates a Semaphore. The count parameter must be a positive
// number. A timeout of zero means that there is no timeout.
func NewSemaphore(count int, timeout time.Duration) *Semaphore {
	sem := &Semaphore{
		slots:   make(chan struct{}, count),
		timeout: timeout,
	}
	for i := 0; i < count; i++ {
		sem.slots <- struct{}{}
	}
	return sem
}

// Acquire returns true on successful acquisition, and
// false on a timeout.
func (sem *Semaphore) Acquire() bool {
	if sem.timeout == 0 {
		<-sem.slots
		return true
	}
	select {
	case <-sem.slots:
		return true
	case <-time.After(sem.timeout):
		return false
	}
}

// Release releases the acquired semaphore. You must
// not release more than the number of semaphores you've
// acquired.
func (sem *Semaphore) Release() {
	sem.slots <- struct{}{}
}
