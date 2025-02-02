// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"fmt"
	"sync"
)

// testStateRecorder is used to record state changes for testing, to help debug async behaviors
type testStateRecorder struct {
	records []string
	mu      sync.Mutex
}

var testRecorder = &testStateRecorder{}

func (t *testStateRecorder) Record(format string, args ...any) {
	t.mu.Lock()
	t.records = append(t.records, fmt.Sprintf(format, args...))
	if len(t.records) > 1000 {
		t.records = t.records[len(t.records)-1000:]
	}
	t.mu.Unlock()
}

func (t *testStateRecorder) Records() []string {
	t.mu.Lock()
	r := make([]string, len(t.records))
	copy(r, t.records)
	t.mu.Unlock()
	return r
}

func (t *testStateRecorder) Reset() {
	t.mu.Lock()
	t.records = nil
	t.mu.Unlock()
}
