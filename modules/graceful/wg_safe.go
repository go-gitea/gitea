// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package graceful

import (
	"sync"
)

// SafeWaitGroup is a small wrapper around sync.WaitGroup that prevents
// new Adds after Shutdown has been requested. It prevents the "WaitGroup
// is reused before previous Wait has returned" panic by gating Add calls.
type SafeWaitGroup struct {
	mu       sync.Mutex
	wg       sync.WaitGroup
	shutting bool
}

// AddIfRunning attempts to add one to the waitgroup. It returns true if
// the add succeeded; false if shutdown has already started and add was rejected.
func (s *SafeWaitGroup) AddIfRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shutting {
		return false
	}
	s.wg.Add(1)
	return true
}

// Done decrements the wait group counter.
// Call only if AddIfRunning returned true previously.
func (s *SafeWaitGroup) Done() {
	s.wg.Done()
}

// Wait waits for the waitgroup to complete.
func (s *SafeWaitGroup) Wait() {
	s.wg.Wait()
}

// Shutdown marks the group as shutting and then waits for all existing
// routines to finish. After Shutdown returns, AddIfRunning will return false.
func (s *SafeWaitGroup) Shutdown() {
	s.mu.Lock()
	s.shutting = true
	s.mu.Unlock()
	s.wg.Wait()
}
