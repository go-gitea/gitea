// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package globallock

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocker(t *testing.T) {
	t.Run("redis", func(t *testing.T) {
		if os.Getenv("CI") == "" {
			t.Skip("Skip test for local development")
			return
		}
		testLocker(t, NewRedisLocker("redis://127.0.0.1:6379/0"))
	})
	t.Run("memory", func(t *testing.T) {
		testLocker(t, NewMemoryLocker())
	})
}

func testLocker(t *testing.T, locker Locker) {
	t.Run("lock", func(t *testing.T) {
		parentCtx := context.Background()
		ctx, unlock, err := locker.Lock(parentCtx, "test")
		defer unlock()

		assert.NotEqual(t, parentCtx, ctx) // new context should be returned
		assert.NoError(t, err)

		func() {
			parentCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			ctx, unlock, err := locker.Lock(parentCtx, "test")
			defer unlock()

			assert.Error(t, err)
			assert.Equal(t, parentCtx, ctx) // should return the same context
		}()

		unlock()
		assert.Error(t, ctx.Err())
		unlock() // should be safe to call multiple times

		func() {
			_, unlock, err := locker.Lock(context.Background(), "test")
			defer unlock()

			assert.NoError(t, err)
		}()
	})

	t.Run("try lock", func(t *testing.T) {
		parentCtx := context.Background()
		ok, ctx, unlock, err := locker.TryLock(parentCtx, "test")
		defer unlock()

		assert.True(t, ok)
		assert.NotEqual(t, parentCtx, ctx) // new context should be returned
		assert.NoError(t, err)

		func() {
			parentCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			ok, ctx, unlock, err := locker.TryLock(parentCtx, "test")
			defer unlock()

			assert.False(t, ok)
			assert.NoError(t, err)
			assert.Equal(t, parentCtx, ctx) // should return the same context
		}()

		unlock()
		assert.Error(t, ctx.Err())
		unlock() // should be safe to call multiple times

		func() {
			ok, _, unlock, _ := locker.TryLock(context.Background(), "test")
			defer unlock()

			assert.True(t, ok)
		}()
	})

	t.Run("wait and acquired", func(t *testing.T) {
		ctx := context.Background()
		ctx, unlock, err := locker.Lock(ctx, "test")
		require.NoError(t, err)

		wg := &sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			started := time.Now()
			_, unlock, err := locker.Lock(context.Background(), "test") // should be blocked for seconds
			defer unlock()
			assert.Greater(t, time.Since(started), time.Second)
			assert.NoError(t, err)
		}()

		time.Sleep(2 * time.Second)
		unlock()

		wg.Wait()
	})
}
