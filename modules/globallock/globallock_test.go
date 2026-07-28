// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package globallock

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLockAndDo(t *testing.T) {
	t.Run("redis", func(t *testing.T) {
		locker := newTestRedisLocker(t)
		defaultLocker.Store(new(locker))
		testLockAndDo(t)
		require.NoError(t, locker.(*redisLocker).Close())
	})
	t.Run("memory", func(t *testing.T) {
		defaultLocker.Store(new(NewMemoryLocker()))
		testLockAndDo(t)
	})
}

func testLockAndDo(t *testing.T) {
	const concurrency = 50

	ctx := t.Context()
	count := 0
	wg := sync.WaitGroup{}
	for range concurrency {
		wg.Go(func() {
			err := LockAndDo(ctx, "test", func(ctx context.Context) error {
				count++
				// It's impossible to acquire the lock inner the function
				ok, err := TryLockAndDo(ctx, "test", func(ctx context.Context) error {
					assert.Fail(t, "should not acquire the lock")
					return nil
				})
				assert.False(t, ok)
				assert.NoError(t, err)
				return nil
			})
			assert.NoError(t, err)
		})
	}
	wg.Wait()

	assert.Equal(t, concurrency, count)
}
