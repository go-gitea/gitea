// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func runWorkerPoolQueue[T any](q *WorkerPoolQueue[T]) func() {
	go q.Run()
	return func() {
		q.ShutdownWait(1 * time.Second)
	}
}

func TestWorkerPoolQueueUnhandled(t *testing.T) {
	oldUnhandledItemRequeueDuration := unhandledItemRequeueDuration.Load()
	unhandledItemRequeueDuration.Store(0)
	defer unhandledItemRequeueDuration.Store(oldUnhandledItemRequeueDuration)

	mu := sync.Mutex{}

	test := func(t *testing.T, queueSetting setting.QueueSettings) {
		queueSetting.Length = 100
		queueSetting.Type = "channel"
		queueSetting.Datadir = t.TempDir() + "/test-queue"
		m := map[int]int{}

		// odds are handled once, evens are handled twice
		handler := func(items ...int) (unhandled []int) {
			testRecorder.Record("handle:%v", items)
			for _, item := range items {
				mu.Lock()
				if item%2 == 0 && m[item] == 0 {
					unhandled = append(unhandled, item)
				}
				m[item]++
				mu.Unlock()
			}
			return unhandled
		}

		q, _ := newWorkerPoolQueueForTest("test-workpoolqueue", queueSetting, handler, false)
		stop := runWorkerPoolQueue(q)
		for i := 0; i < queueSetting.Length; i++ {
			testRecorder.Record("push:%v", i)
			assert.NoError(t, q.Push(i))
		}
		assert.NoError(t, q.FlushWithContext(t.Context(), 0))
		stop()

		ok := true
		for i := 0; i < queueSetting.Length; i++ {
			if i%2 == 0 {
				ok = ok && assert.EqualValues(t, 2, m[i], "test %s: item %d", t.Name(), i)
			} else {
				ok = ok && assert.EqualValues(t, 1, m[i], "test %s: item %d", t.Name(), i)
			}
		}
		if !ok {
			t.Logf("m: %v", m)
			t.Logf("records: %v", testRecorder.Records())
		}
		testRecorder.Reset()
	}

	runCount := 2 // we can run these tests even hundreds times to see its stability
	t.Run("1/1", func(t *testing.T) {
		for i := 0; i < runCount; i++ {
			test(t, setting.QueueSettings{BatchLength: 1, MaxWorkers: 1})
		}
	})
	t.Run("3/1", func(t *testing.T) {
		for i := 0; i < runCount; i++ {
			test(t, setting.QueueSettings{BatchLength: 3, MaxWorkers: 1})
		}
	})
	t.Run("4/5", func(t *testing.T) {
		for i := 0; i < runCount; i++ {
			test(t, setting.QueueSettings{BatchLength: 4, MaxWorkers: 5})
		}
	})
}

func TestWorkerPoolQueuePersistence(t *testing.T) {
	runCount := 2 // we can run these tests even hundreds times to see its stability
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

func testWorkerPoolQueuePersistence(t *testing.T, queueSetting setting.QueueSettings) {
	testCount := queueSetting.Length
	queueSetting.Type = "level"
	queueSetting.Datadir = t.TempDir() + "/test-queue"

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

		q, _ := newWorkerPoolQueueForTest("pr_patch_checker_test", queueSetting, testHandler, true)
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

		q, _ := newWorkerPoolQueueForTest("pr_patch_checker_test", queueSetting, testHandler, true)
		stop := runWorkerPoolQueue(q)
		assert.NoError(t, q.FlushWithContext(t.Context(), 0))
		stop()
	}

	q2() // restart the queue to continue to execute the tasks in it

	assert.NotEmpty(t, tasksQ1)
	assert.NotEmpty(t, tasksQ2)
	assert.EqualValues(t, testCount, len(tasksQ1)+len(tasksQ2))
}

func TestWorkerPoolQueueActiveWorkers(t *testing.T) {
	defer test.MockVariableValue(&workerIdleDuration, 300*time.Millisecond)()

	handler := func(items ...int) (unhandled []int) {
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	q, _ := newWorkerPoolQueueForTest("test-workpoolqueue", setting.QueueSettings{Type: "channel", BatchLength: 1, MaxWorkers: 1, Length: 100}, handler, false)
	stop := runWorkerPoolQueue(q)
	for i := 0; i < 5; i++ {
		assert.NoError(t, q.Push(i))
	}

	time.Sleep(50 * time.Millisecond)
	assert.EqualValues(t, 1, q.GetWorkerNumber())
	assert.EqualValues(t, 1, q.GetWorkerActiveNumber())
	time.Sleep(500 * time.Millisecond)
	assert.EqualValues(t, 1, q.GetWorkerNumber())
	assert.EqualValues(t, 0, q.GetWorkerActiveNumber())
	time.Sleep(workerIdleDuration)
	assert.EqualValues(t, 1, q.GetWorkerNumber()) // there is at least one worker after the queue begins working
	stop()

	q, _ = newWorkerPoolQueueForTest("test-workpoolqueue", setting.QueueSettings{Type: "channel", BatchLength: 1, MaxWorkers: 3, Length: 100}, handler, false)
	stop = runWorkerPoolQueue(q)
	for i := 0; i < 15; i++ {
		assert.NoError(t, q.Push(i))
	}

	time.Sleep(50 * time.Millisecond)
	assert.EqualValues(t, 3, q.GetWorkerNumber())
	assert.EqualValues(t, 3, q.GetWorkerActiveNumber())
	time.Sleep(500 * time.Millisecond)
	assert.EqualValues(t, 3, q.GetWorkerNumber())
	assert.EqualValues(t, 0, q.GetWorkerActiveNumber())
	time.Sleep(workerIdleDuration)
	assert.EqualValues(t, 1, q.GetWorkerNumber()) // there is at least one worker after the queue begins working
	stop()
}

func TestWorkerPoolQueueShutdown(t *testing.T) {
	oldUnhandledItemRequeueDuration := unhandledItemRequeueDuration.Load()
	unhandledItemRequeueDuration.Store(int64(100 * time.Millisecond))
	defer unhandledItemRequeueDuration.Store(oldUnhandledItemRequeueDuration)

	// simulate a slow handler, it doesn't handle any item (all items will be pushed back to the queue)
	handlerCalled := make(chan struct{})
	handler := func(items ...int) (unhandled []int) {
		if items[0] == 0 {
			close(handlerCalled)
		}
		time.Sleep(400 * time.Millisecond)
		return items
	}

	qs := setting.QueueSettings{Type: "level", Datadir: t.TempDir() + "/queue", BatchLength: 3, MaxWorkers: 4, Length: 20}
	q, _ := newWorkerPoolQueueForTest("test-workpoolqueue", qs, handler, false)
	stop := runWorkerPoolQueue(q)
	for i := 0; i < qs.Length; i++ {
		assert.NoError(t, q.Push(i))
	}
	<-handlerCalled
	time.Sleep(200 * time.Millisecond) // wait for a while to make sure all workers are active
	assert.EqualValues(t, 4, q.GetWorkerActiveNumber())
	stop() // stop triggers shutdown
	assert.EqualValues(t, 0, q.GetWorkerActiveNumber())

	// no item was ever handled, so we still get all of them again
	q, _ = newWorkerPoolQueueForTest("test-workpoolqueue", qs, handler, false)
	assert.EqualValues(t, 20, q.GetQueueItemNumber())
}

func TestWorkerPoolQueueWorkerIdleReset(t *testing.T) {
	defer test.MockVariableValue(&workerIdleDuration, 10*time.Millisecond)()
	defer mockBackoffDuration(5 * time.Millisecond)()

	var q *WorkerPoolQueue[int]
	var handledCount atomic.Int32
	var hasOnlyOneWorkerRunning atomic.Bool
	handler := func(items ...int) (unhandled []int) {
		handledCount.Add(int32(len(items)))
		// make each work have different duration, and check the active worker number periodically
		var activeNums []int
		for i := 0; i < 5-items[0]%2; i++ {
			time.Sleep(workerIdleDuration * 2)
			activeNums = append(activeNums, q.GetWorkerActiveNumber())
		}
		// When the queue never becomes empty, the existing workers should keep working
		// It is not 100% true at the moment because the data-race in workergroup.go is not resolved, see that TODO */
		// If the "active worker numbers" is like [2 2 ... 1 1], it means that an existing worker exited and the no new worker is started.
		if slices.Equal([]int{1, 1}, activeNums[len(activeNums)-2:]) {
			hasOnlyOneWorkerRunning.Store(true)
		}
		return nil
	}
	q, _ = newWorkerPoolQueueForTest("test-workpoolqueue", setting.QueueSettings{Type: "channel", BatchLength: 1, MaxWorkers: 2, Length: 100}, handler, false)
	stop := runWorkerPoolQueue(q)
	for i := 0; i < 100; i++ {
		assert.NoError(t, q.Push(i))
	}
	time.Sleep(500 * time.Millisecond)
	assert.Greater(t, int(handledCount.Load()), 4) // make sure there are enough items handled during the test
	assert.False(t, hasOnlyOneWorkerRunning.Load(), "a slow handler should not block other workers from starting")
	stop()
}
