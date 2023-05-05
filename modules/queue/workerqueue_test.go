// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func runWorkerPoolQueue[T any](q *WorkerPoolQueue[T]) func() {
	var stop func()
	started := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		q.Run(func(f func()) { stop = f; close(started) }, nil)
		close(stopped)
	}()
	<-started
	return func() {
		stop()
		<-stopped
	}
}

func TestWorkerPoolQueueUnhandled(t *testing.T) {
	oldUnhandledItemRequeueDuration := unhandledItemRequeueDuration
	unhandledItemRequeueDuration = 0
	defer func() {
		unhandledItemRequeueDuration = oldUnhandledItemRequeueDuration
	}()

	mu := sync.Mutex{}

	test := func(iniCfg setting.QueueSettings) {
		iniCfg.Length = 100
		iniCfg.Datadir = t.TempDir() + "/test-queue"
		m := map[int]int{}
		// odds are handled once, evens are handled twice
		handler := func(items ...int) (unhandled []int) {
			for _, item := range items {
				mu.Lock()
				if item%2 == 0 && m[item] != 1 {
					unhandled = append(unhandled, item)
				}
				m[item]++
				mu.Unlock()
			}
			return unhandled
		}

		q := NewWorkerPoolQueueByIniConfig("test-workpoolqueue", iniCfg, handler, false)
		stop := runWorkerPoolQueue(q)
		for i := 0; i < iniCfg.Length; i++ {
			assert.NoError(t, q.Push(i))
		}
		assert.NoError(t, q.FlushWithContext(context.Background(), 0))
		stop()

		for i := 0; i < iniCfg.Length; i++ {
			assert.EqualValues(t, i%2, m[i]%2)
		}
	}

	runCount := 2
	t.Run("1/1", func(t *testing.T) {
		for i := 0; i < runCount; i++ {
			test(setting.QueueSettings{BatchLength: 1, MaxWorkers: 1})
		}
	})
	t.Run("3/1", func(t *testing.T) {
		for i := 0; i < runCount; i++ {
			test(setting.QueueSettings{BatchLength: 3, MaxWorkers: 1})
		}
	})
	t.Run("4/5", func(t *testing.T) {
		for i := 0; i < runCount; i++ {
			test(setting.QueueSettings{BatchLength: 4, MaxWorkers: 5})
		}
	})
}

func TestWorkerPoolQueuePersistence(t *testing.T) {
	runCount := 2 // we can run these tests even 100 times to see its stability
	t.Run("1/1", func(t *testing.T) {
		for i := 0; i < runCount; i++ {
			testWorkerPoolQueuePersistence(t, setting.QueueSettings{BatchLength: 1, MaxWorkers: 1, Length: 100})
		}
	})
	t.Run("3/1", func(t *testing.T) {
		for i := 0; i < runCount; i++ {
			testWorkerPoolQueuePersistence(t, setting.QueueSettings{BatchLength: 3, MaxWorkers: 1, Length: 100})
		}
	})
	t.Run("4/5", func(t *testing.T) {
		for i := 0; i < runCount; i++ {
			testWorkerPoolQueuePersistence(t, setting.QueueSettings{BatchLength: 4, MaxWorkers: 5, Length: 100})
		}
	})
}

func testWorkerPoolQueuePersistence(t *testing.T, iniCfg setting.QueueSettings) {
	testCount := iniCfg.Length
	iniCfg.Datadir = t.TempDir() + "/test-queue"

	mu := sync.Mutex{}

	var tasksQ1, tasksQ2 []string
	q1 := func() {
		startWhenAllReady := make(chan struct{}) // only start data consuming when the "testCount" tasks are all pushed into queue
		stopAt20Shutdown := make(chan struct{})  // stop and shutdown at the 20th item

		testHandler := func(data ...string) []string {
			<-startWhenAllReady
			time.Sleep(10 * time.Millisecond)
			for _, s := range data {
				mu.Lock()
				tasksQ1 = append(tasksQ1, s)
				mu.Unlock()

				if s == "task-20" {
					close(stopAt20Shutdown)
				}
			}
			return nil
		}

		q := NewWorkerPoolQueueByIniConfig("pr_patch_checker_test", iniCfg, testHandler, true)
		stop := runWorkerPoolQueue(q)
		for i := 0; i < testCount; i++ {
			_ = q.Push("task-" + strconv.Itoa(i))
		}
		close(startWhenAllReady)
		<-stopAt20Shutdown // it's possible to have more than 20 tasks executed
		stop()
	}

	q1() // run some tasks and shutdown at an intermediate point

	time.Sleep(100 * time.Millisecond) // because the handler in q1 has a slight delay, we need to wait for it to finish

	q2 := func() {
		testHandler := func(data ...string) []string {
			for _, s := range data {
				mu.Lock()
				tasksQ2 = append(tasksQ2, s)
				mu.Unlock()
			}
			return nil
		}

		q := NewWorkerPoolQueueByIniConfig("pr_patch_checker_test", iniCfg, testHandler, true)
		stop := runWorkerPoolQueue(q)
		assert.NoError(t, q.FlushWithContext(context.Background(), 0))
		stop()
	}

	q2() // restart the queue to continue to execute the tasks in it

	assert.NotZero(t, len(tasksQ1))
	assert.NotZero(t, len(tasksQ2))
	assert.EqualValues(t, testCount, len(tasksQ1)+len(tasksQ2))
}
