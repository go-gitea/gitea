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

func TestLockAndDo(t *testing.T) {
	t.Run("redis", func(t *testing.T) {
		url := "redis://127.0.0.1:6379/0"
		if os.Getenv("CI") == "" {
			// Make it possible to run tests against a local redis instance
			url = os.Getenv("TEST_REDIS_URL")
			if url == "" {
				t.Skip("TEST_REDIS_URL not set and not running in CI")
				return
			}
		}

		initOnce = sync.Once{}
		initFunc = func() {
			defaultLocker = NewRedisLocker(url)
		}

		testLockAndDo(t)
	})
	t.Run("memory", func(t *testing.T) {
		initOnce = sync.Once{}
		initFunc = func() {
			defaultLocker = NewMemoryLocker()
		}

		testLockAndDo(t)
	})
}

func testLockAndDo(t *testing.T) {
	const (
		duration    = 2 * time.Second
		concurrency = 100
	)

	ctx := context.Background()
	count := 0
	wg := sync.WaitGroup{}
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			err := LockAndDo(ctx, "test", func(ctx context.Context) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(duration / concurrency):
					count++
				}
				return nil
			})
			require.NoError(t, err)
		}()
	}

	ok, err := TryLockAndDo(ctx, "test", func(ctx context.Context) error {
		return nil
	})
	assert.False(t, ok)
	assert.NoError(t, err)

	wg.Wait()

	ok, err = TryLockAndDo(ctx, "test", func(ctx context.Context) error {
		return nil
	})
	assert.True(t, ok)
	assert.NoError(t, err)

	assert.Equal(t, concurrency, count)
}
