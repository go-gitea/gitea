// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"sync"
	"time"
)

// StopTimer is a utility function to safely stop a time.Timer and clean its channel
func StopTimer(t *time.Timer) bool {
	stopped := t.Stop()
	if !stopped {
		select {
		case <-t.C:
		default:
		}
	}
	return stopped
}

func Debounce(d time.Duration) func(f func()) {
	type debouncer struct {
		mu sync.Mutex
		t  *time.Timer
	}
	db := &debouncer{}

	return func(f func()) {
		db.mu.Lock()
		defer db.mu.Unlock()

		if db.t != nil {
			db.t.Stop()
		}
		var trigger *time.Timer
		trigger = time.AfterFunc(d, func() {
			db.mu.Lock()
			defer db.mu.Unlock()
			if trigger == db.t {
				f()
				db.t = nil
			}
		})
		db.t = trigger
	}
}
