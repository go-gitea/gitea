// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import "sync"

var taskVersionWake = struct {
	mu sync.Mutex
	ch chan struct{}
}{ch: make(chan struct{})}

// TasksVersionWakeChannel must be read before the version to avoid missing a wake.
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

func (v *ActionTasksVersion) AfterInsert() {
	v.AfterUpdate()
}

func (v *ActionTasksVersion) AfterUpdate() {
	if v.OwnerID == 0 && v.RepoID == 0 {
		signalTaskVersionWake()
	}
}
