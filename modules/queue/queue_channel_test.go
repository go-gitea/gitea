// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"os"
	"sync"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/log"

	"github.com/stretchr/testify/assert"
)

func TestChannelQueue(t *testing.T) {
	handleChan := make(chan *testData)
	handle := func(data ...Data) []Data {
		for _, datum := range data {
			testDatum := datum.(*testData)
			handleChan <- testDatum
		}
		return nil
	}

	nilFn := func(_ func()) {}

	queue, err := NewChannelQueue(handle,
		ChannelQueueConfiguration{
			WorkerPoolConfiguration: WorkerPoolConfiguration{
				QueueLength:  0,
				MaxWorkers:   10,
				BlockTimeout: 1 * time.Second,
				BoostTimeout: 5 * time.Minute,
				BoostWorkers: 5,
				Name:         "TestChannelQueue",
			},
			Workers: 0,
		}, &testData{})
	assert.NoError(t, err)

	assert.Equal(t, 5, queue.(*ChannelQueue).WorkerPool.boostWorkers)

	go queue.Run(nilFn, nilFn)

	test1 := testData{"A", 1}
	go queue.Push(&test1)
	result1 := <-handleChan
	assert.Equal(t, test1.TestString, result1.TestString)
	assert.Equal(t, test1.TestInt, result1.TestInt)

	err = queue.Push(test1)
	assert.Error(t, err)
}

func TestChannelQueue_Batch(t *testing.T) {
	handleChan := make(chan *testData)
	handle := func(data ...Data) []Data {
		assert.True(t, len(data) == 2)
		for _, datum := range data {
			testDatum := datum.(*testData)
			handleChan <- testDatum
		}
		return nil
	}

	nilFn := func(_ func()) {}

	queue, err := NewChannelQueue(handle,
		ChannelQueueConfiguration{
			WorkerPoolConfiguration: WorkerPoolConfiguration{
				QueueLength:  20,
				BatchLength:  2,
				BlockTimeout: 0,
				BoostTimeout: 0,
				BoostWorkers: 0,
				MaxWorkers:   10,
			},
			Workers: 1,
		}, &testData{})
	assert.NoError(t, err)

	go queue.Run(nilFn, nilFn)

	test1 := testData{"A", 1}
	test2 := testData{"B", 2}

	queue.Push(&test1)
	go queue.Push(&test2)

	result1 := <-handleChan
	assert.Equal(t, test1.TestString, result1.TestString)
	assert.Equal(t, test1.TestInt, result1.TestInt)

	result2 := <-handleChan
	assert.Equal(t, test2.TestString, result2.TestString)
	assert.Equal(t, test2.TestInt, result2.TestInt)

	err = queue.Push(test1)
	assert.Error(t, err)
}

func TestChannelQueue_Pause(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping because test is flaky on CI")
	}
	lock := sync.Mutex{}
	var queue Queue
	var err error
	pushBack := false
	handleChan := make(chan *testData)
	handle := func(data ...Data) []Data {
		lock.Lock()
		if pushBack {
			if pausable, ok := queue.(Pausable); ok {
				pausable.Pause()
			}
			lock.Unlock()
			return data
		}
		lock.Unlock()

		for _, datum := range data {
			testDatum := datum.(*testData)
			handleChan <- testDatum
		}
		return nil
	}

	queueShutdown := []func(){}
	queueTerminate := []func(){}

	terminated := make(chan struct{})

	queue, err = NewChannelQueue(handle,
		ChannelQueueConfiguration{
			WorkerPoolConfiguration: WorkerPoolConfiguration{
				QueueLength:  20,
				BatchLength:  1,
				BlockTimeout: 0,
				BoostTimeout: 0,
				BoostWorkers: 0,
				MaxWorkers:   10,
			},
			Workers: 1,
		}, &testData{})
	assert.NoError(t, err)

	go func() {
		queue.Run(func(shutdown func()) {
			lock.Lock()
			defer lock.Unlock()
			queueShutdown = append(queueShutdown, shutdown)
		}, func(terminate func()) {
			lock.Lock()
			defer lock.Unlock()
			queueTerminate = append(queueTerminate, terminate)
		})
		close(terminated)
	}()

	// Shutdown and Terminate in defer
	defer func() {
		lock.Lock()
		callbacks := make([]func(), len(queueShutdown))
		copy(callbacks, queueShutdown)
		lock.Unlock()
		for _, callback := range callbacks {
			callback()
		}
		lock.Lock()
		log.Info("Finally terminating")
		callbacks = make([]func(), len(queueTerminate))
		copy(callbacks, queueTerminate)
		lock.Unlock()
		for _, callback := range callbacks {
			callback()
		}
	}()

	test1 := testData{"A", 1}
	test2 := testData{"B", 2}
	queue.Push(&test1)

	pausable, ok := queue.(Pausable)
	if !assert.True(t, ok) {
		return
	}
	result1 := <-handleChan
	assert.Equal(t, test1.TestString, result1.TestString)
	assert.Equal(t, test1.TestInt, result1.TestInt)

	pausable.Pause()

	paused, _ := pausable.IsPausedIsResumed()

	select {
	case <-paused:
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "Queue is not paused")
		return
	}

	queue.Push(&test2)

	var result2 *testData
	select {
	case result2 = <-handleChan:
		assert.Fail(t, "handler chan should be empty")
	case <-time.After(100 * time.Millisecond):
	}

	assert.Nil(t, result2)

	pausable.Resume()
	_, resumed := pausable.IsPausedIsResumed()

	select {
	case <-resumed:
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "Queue should be resumed")
	}

	select {
	case result2 = <-handleChan:
	case <-time.After(500 * time.Millisecond):
		assert.Fail(t, "handler chan should contain test2")
	}

	assert.Equal(t, test2.TestString, result2.TestString)
	assert.Equal(t, test2.TestInt, result2.TestInt)

	lock.Lock()
	pushBack = true
	lock.Unlock()

	_, resumed = pausable.IsPausedIsResumed()

	select {
	case <-resumed:
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "Queue is not resumed")
		return
	}

	queue.Push(&test1)
	paused, _ = pausable.IsPausedIsResumed()

	select {
	case <-paused:
	case <-handleChan:
		assert.Fail(t, "handler chan should not contain test1")
		return
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "queue should be paused")
		return
	}

	lock.Lock()
	pushBack = false
	lock.Unlock()

	paused, _ = pausable.IsPausedIsResumed()

	select {
	case <-paused:
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "Queue is not paused")
		return
	}

	pausable.Resume()
	_, resumed = pausable.IsPausedIsResumed()

	select {
	case <-resumed:
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "Queue should be resumed")
	}

	select {
	case result1 = <-handleChan:
	case <-time.After(500 * time.Millisecond):
		assert.Fail(t, "handler chan should contain test1")
	}
	assert.Equal(t, test1.TestString, result1.TestString)
	assert.Equal(t, test1.TestInt, result1.TestInt)

	lock.Lock()
	callbacks := make([]func(), len(queueShutdown))
	copy(callbacks, queueShutdown)
	queueShutdown = queueShutdown[:0]
	lock.Unlock()
	// Now shutdown the queue
	for _, callback := range callbacks {
		callback()
	}

	// terminate the queue
	lock.Lock()
	callbacks = make([]func(), len(queueTerminate))
	copy(callbacks, queueTerminate)
	queueShutdown = queueTerminate[:0]
	lock.Unlock()
	for _, callback := range callbacks {
		callback()
	}
	select {
	case <-terminated:
	case <-time.After(10 * time.Second):
		assert.Fail(t, "Queue should have terminated")
		return
	}
}
