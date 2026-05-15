// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"sync"
	"testing"

	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func assertChannelOpen(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal(msg)
	default:
	}
}

func assertChannelClosed(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
	default:
		t.Fatal(msg)
	}
}

func TestTasksVersionWake_SignalClosesChannel(t *testing.T) {
	ch := TasksVersionWakeChannel()
	assertChannelOpen(t, ch, "channel should be open before signal")

	signalTaskVersionWake()
	assertChannelClosed(t, ch, "channel should be closed after signal")

	assertChannelOpen(t, TasksVersionWakeChannel(), "replacement channel should be open")
}

func TestIncreaseTaskVersion_SignalsWake(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ch := TasksVersionWakeChannel()
	assert.NoError(t, IncreaseTaskVersion(t.Context(), 0, 0))
	assertChannelClosed(t, ch, "expected IncreaseTaskVersion to close the wake channel")
}

// TestTasksVersionWake_ConcurrentSafe runs concurrent get/signal calls;
// run with `-race` to catch regressions in the mutex.
func TestTasksVersionWake_ConcurrentSafe(t *testing.T) {
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() { _ = TasksVersionWakeChannel() })
		wg.Go(func() { signalTaskVersionWake() })
	}
	wg.Wait()

	assertChannelOpen(t, TasksVersionWakeChannel(), "a fresh subscription should be open until the next signal")
}
