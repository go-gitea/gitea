// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package graceful

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSafeWaitGroup_AddIfRunning(t *testing.T) {
	var swg SafeWaitGroup

	// Test that AddIfRunning succeeds before shutdown
	assert.True(t, swg.AddIfRunning(), "AddIfRunning should succeed before shutdown")
	swg.Done()

	// Test that AddIfRunning fails after shutdown
	swg.Shutdown()
	assert.False(t, swg.AddIfRunning(), "AddIfRunning should fail after shutdown")
}

func TestSafeWaitGroup_Shutdown(t *testing.T) {
	var swg SafeWaitGroup

	// Add some work
	assert.True(t, swg.AddIfRunning())
	assert.True(t, swg.AddIfRunning())

	// Complete work in goroutines
	go func() {
		time.Sleep(50 * time.Millisecond)
		swg.Done()
	}()
	go func() {
		time.Sleep(100 * time.Millisecond)
		swg.Done()
	}()

	// Shutdown should wait for all work to complete
	start := time.Now()
	swg.Shutdown()
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond, "Shutdown should wait for all work")
}

func TestSafeWaitGroup_ConcurrentAddAndShutdown(t *testing.T) {
	var swg SafeWaitGroup
	var addCount atomic.Int32
	var successCount atomic.Int32

	// Start many goroutines trying to add
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for range numGoroutines {
		go func() {
			defer wg.Done()
			if swg.AddIfRunning() {
				addCount.Add(1)
				time.Sleep(10 * time.Millisecond)
				swg.Done()
				successCount.Add(1)
			}
		}()
	}

	// Give some goroutines time to add
	time.Sleep(5 * time.Millisecond)

	// Now shutdown
	swg.Shutdown()

	// Wait for all goroutines to finish
	wg.Wait()

	// All adds that succeeded should have completed
	assert.Equal(t, addCount.Load(), successCount.Load(), "All successful adds should complete")

	// After shutdown, no new adds should succeed
	assert.False(t, swg.AddIfRunning(), "No adds should succeed after shutdown")
}

func TestSafeWaitGroup_MultipleShutdowns(t *testing.T) {
	var swg SafeWaitGroup

	// First shutdown
	swg.Shutdown()

	// Second shutdown should not panic
	assert.NotPanics(t, func() {
		swg.Shutdown()
	}, "Multiple shutdowns should not panic")
}

func TestSafeWaitGroup_WaitWithoutAdd(t *testing.T) {
	var swg SafeWaitGroup

	// Wait without any adds should not block
	done := make(chan struct{})
	go func() {
		swg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Wait should not block when no work is added")
	}
}

func TestSafeWaitGroup_PreventsPanic(t *testing.T) {
	var swg SafeWaitGroup

	// This pattern would cause a panic with sync.WaitGroup:
	// Add -> Wait (in goroutine) -> Add (would panic)

	assert.True(t, swg.AddIfRunning())

	go func() {
		time.Sleep(10 * time.Millisecond)
		swg.Done()
	}()

	// Start shutdown which will wait
	go swg.Shutdown()

	// Give shutdown time to start
	time.Sleep(5 * time.Millisecond)

	// This should not panic, just return false
	assert.NotPanics(t, func() {
		result := swg.AddIfRunning()
		assert.False(t, result, "Add should be rejected during shutdown")
	}, "AddIfRunning should not panic during shutdown")
}

func TestSafeWaitGroup_RaceCondition(t *testing.T) {
	// This test is designed to catch race conditions
	// Run with: go test -race
	var swg SafeWaitGroup
	var wg sync.WaitGroup

	const numWorkers = 50
	wg.Add(numWorkers * 2) // workers + shutdown goroutines

	// Start workers
	for range numWorkers {
		go func() {
			defer wg.Done()
			for range 10 {
				if swg.AddIfRunning() {
					time.Sleep(time.Millisecond)
					swg.Done()
				}
			}
		}()
	}

	// Start shutdown attempts
	for range numWorkers {
		go func() {
			defer wg.Done()
			time.Sleep(5 * time.Millisecond)
			swg.Shutdown()
		}()
	}

	wg.Wait()
}
