// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"os"
	"strconv"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerPoolQueueRedis(t *testing.T) {
	var redisServerCmdProcess *os.Process
	defer func() {
		if redisServerCmdProcess != nil {
			_ = redisServerCmdProcess.Signal(os.Interrupt)
			_, _ = redisServerCmdProcess.Wait()
		}
	}()

	// Reusing waitRedisReady and redisServerCmd from base_redis_test.go
	if !waitRedisReady("redis://127.0.0.1:6379/0", 0) {
		cmd := redisServerCmd(t)
		if cmd == nil && os.Getenv("CI") == "" {
			t.Skip("redis-server not found")
			return
		}
		if cmd != nil {
			assert.NoError(t, cmd.Start())
			redisServerCmdProcess = cmd.Process
			require.True(t, waitRedisReady("redis://127.0.0.1:6379/0", 5*time.Second), "start redis-server")
		}
	}

	// Define a test similar to testWorkerPoolQueuePersistence but for Redis
	testRedisPersistence := func(t *testing.T, queueSetting setting.QueueSettings) {
		queueSetting.Type = "redis"
		queueSetting.ConnStr = "redis://127.0.0.1:6379/0"
		// QueueName and SetName are needed for Redis to avoid collisions between runs/tests
		queueSetting.QueueName = "_test_redis_queue_" + t.Name()
		queueSetting.SetName = "_test_redis_set_" + t.Name()

		// Cleanup before test
		base, err := newBaseRedisSimple(toBaseConfig("test-redis-queue", queueSetting))
		require.NoError(t, err)
		_ = base.RemoveAll(t.Context())
		_ = base.Close()

		// Reuse the logic from testWorkerPoolQueuePersistence
		// We need to slightly adapt it because testWorkerPoolQueuePersistence hardcodes Type="level"
		// So we will implement a simplified version of that test here.

		testCount := queueSetting.Length

		// q1: Push items and consume some
		var tasksQ1 []string
		q1Done := make(chan struct{})
		
		q1Handler := func(data ...string) []string {
			for _, s := range data {
				tasksQ1 = append(tasksQ1, s)
				if len(tasksQ1) == 20 {
					close(q1Done)
				}
			}
			return nil
		}

		q1, err := newWorkerPoolQueueForTest("test-redis-queue", queueSetting, q1Handler, true)
		require.NoError(t, err)
		
		stop1 := runWorkerPoolQueue(q1)
		
		for i := 0; i < testCount; i++ {
			// using unique items
			_ = q1.Push("task-" + t.Name() + "-" + strconv.Itoa(i))
		}

		// Wait until we processed some items
		select {
		case <-q1Done:
		case <-time.After(5 * time.Second):
			// Proceed even if timeout, might be slow machine
		}
		
		stop1() // Stop q1, should shut down gracefully
		
		// q2: Restart queue and consume the rest
		var tasksQ2 []string
		q2Handler := func(data ...string) []string {
			for _, s := range data {
				tasksQ2 = append(tasksQ2, s)
			}
			return nil
		}

		q2, err := newWorkerPoolQueueForTest("test-redis-queue", queueSetting, q2Handler, true)
		require.NoError(t, err)

		stop2 := runWorkerPoolQueue(q2)
		// Flush to ensure all remaining items are processed
		err = q2.FlushWithContext(t.Context(), 5*time.Second)
		assert.NoError(t, err)
		stop2()

		total := len(tasksQ1) + len(tasksQ2)
		assert.Equal(t, testCount, total, "Should process all items across restarts")
		
		// Cleanup after test
		_ = q2.RemoveAllItems(t.Context())
	}

	t.Run("Persistence", func(t *testing.T) {
		testRedisPersistence(t, setting.QueueSettings{
			BatchLength: 10, 
			MaxWorkers: 2, 
			Length: 100,
		})
	})
}
