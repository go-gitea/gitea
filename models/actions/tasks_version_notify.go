// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import "sync"

// taskVersionWake broadcasts to FetchTask waiters when any task version is bumped.
var taskVersionWake = struct {
	mu sync.Mutex
	ch chan struct{}
}{ch: make(chan struct{})}

// TasksVersionWakeChannel returns a channel that is closed by the next
// IncreaseTaskVersion. Subscribe before reading the version so a signal
// arriving in between is not lost.
func TasksVersionWakeChannel() <-chan struct{} {
	taskVersionWake.mu.Lock()
	defer taskVersionWake.mu.Unlock()
	return taskVersionWake.ch
}

func signalTaskVersionWake() {
	taskVersionWake.mu.Lock()
	defer taskVersionWake.mu.Unlock()
	close(taskVersionWake.ch)
	taskVersionWake.ch = make(chan struct{})
}
