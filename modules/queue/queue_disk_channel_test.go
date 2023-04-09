// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"sync"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/log"

	"github.com/stretchr/testify/assert"
)

func TestPersistableChannelQueue(t *testing.T) {
	handleChan := make(chan *testData)
	handle := func(data ...Data) []Data {
		for _, datum := range data {
			if datum == nil {
				continue
			}
			testDatum := datum.(*testData)
			handleChan <- testDatum
		}
		return nil
	}

	lock := sync.Mutex{}
	queueShutdown := []func(){}
	queueTerminate := []func(){}

	tmpDir := t.TempDir()

	queue, err := NewPersistableChannelQueue(handle, PersistableChannelQueueConfiguration{
		DataDir:      tmpDir,
		BatchLength:  2,
		QueueLength:  20,
		Workers:      1,
		BoostWorkers: 0,
		MaxWorkers:   10,
		Name:         "test-queue",
	}, &testData{})
	assert.NoError(t, err)

	readyForShutdown := make(chan struct{})
	readyForTerminate := make(chan struct{})

	go queue.Run(func(shutdown func()) {
		lock.Lock()
		defer lock.Unlock()
		select {
		case <-readyForShutdown:
		default:
			close(readyForShutdown)
		}
		queueShutdown = append(queueShutdown, shutdown)
	}, func(terminate func()) {
		lock.Lock()
		defer lock.Unlock()
		select {
		case <-readyForTerminate:
		default:
			close(readyForTerminate)
		}
		queueTerminate = append(queueTerminate, terminate)
	})

	test1 := testData{"A", 1}
	test2 := testData{"B", 2}

	err = queue.Push(&test1)
	assert.NoError(t, err)
	go func() {
		err := queue.Push(&test2)
		assert.NoError(t, err)
	}()

	result1 := <-handleChan
	assert.Equal(t, test1.TestString, result1.TestString)
	assert.Equal(t, test1.TestInt, result1.TestInt)

	result2 := <-handleChan
	assert.Equal(t, test2.TestString, result2.TestString)
	assert.Equal(t, test2.TestInt, result2.TestInt)

	// test1 is a testData not a *testData so will be rejected
	err = queue.Push(test1)
	assert.Error(t, err)

	<-readyForShutdown
	// Now shutdown the queue
	lock.Lock()
	callbacks := make([]func(), len(queueShutdown))
	copy(callbacks, queueShutdown)
	lock.Unlock()
	for _, callback := range callbacks {
		callback()
	}

	// Wait til it is closed
	<-queue.(*PersistableChannelQueue).closed

	err = queue.Push(&test1)
	assert.NoError(t, err)
	err = queue.Push(&test2)
	assert.NoError(t, err)
	select {
	case <-handleChan:
		assert.Fail(t, "Handler processing should have stopped")
	default:
	}

	// terminate the queue
	<-readyForTerminate
	lock.Lock()
	callbacks = make([]func(), len(queueTerminate))
	copy(callbacks, queueTerminate)
	lock.Unlock()
	for _, callback := range callbacks {
		callback()
	}

	select {
	case <-handleChan:
		assert.Fail(t, "Handler processing should have stopped")
	default:
	}

	// Reopen queue
	queue, err = NewPersistableChannelQueue(handle, PersistableChannelQueueConfiguration{
		DataDir:      tmpDir,
		BatchLength:  2,
		QueueLength:  20,
		Workers:      1,
		BoostWorkers: 0,
		MaxWorkers:   10,
		Name:         "test-queue",
	}, &testData{})
	assert.NoError(t, err)

	readyForShutdown = make(chan struct{})
	readyForTerminate = make(chan struct{})

	go queue.Run(func(shutdown func()) {
		lock.Lock()
		defer lock.Unlock()
		select {
		case <-readyForShutdown:
		default:
			close(readyForShutdown)
		}
		queueShutdown = append(queueShutdown, shutdown)
	}, func(terminate func()) {
		lock.Lock()
		defer lock.Unlock()
		select {
		case <-readyForTerminate:
		default:
			close(readyForTerminate)
		}
		queueTerminate = append(queueTerminate, terminate)
	})

	result3 := <-handleChan
	assert.Equal(t, test1.TestString, result3.TestString)
	assert.Equal(t, test1.TestInt, result3.TestInt)

	result4 := <-handleChan
	assert.Equal(t, test2.TestString, result4.TestString)
	assert.Equal(t, test2.TestInt, result4.TestInt)

	<-readyForShutdown
	lock.Lock()
	callbacks = make([]func(), len(queueShutdown))
	copy(callbacks, queueShutdown)
	lock.Unlock()
	for _, callback := range callbacks {
		callback()
	}
	<-readyForTerminate
	lock.Lock()
	callbacks = make([]func(), len(queueTerminate))
	copy(callbacks, queueTerminate)
	lock.Unlock()
	for _, callback := range callbacks {
		callback()
	}
}

func TestPersistableChannelQueue_Pause(t *testing.T) {
	lock := sync.Mutex{}
	var queue Queue
	var err error
	pushBack := false

	handleChan := make(chan *testData)
	handle := func(data ...Data) []Data {
		lock.Lock()
		if pushBack {
			if pausable, ok := queue.(Pausable); ok {
				log.Info("pausing")
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

	tmpDir := t.TempDir()

	queue, err = NewPersistableChannelQueue(handle, PersistableChannelQueueConfiguration{
		DataDir:      tmpDir,
		BatchLength:  2,
		QueueLength:  20,
		Workers:      1,
		BoostWorkers: 0,
		MaxWorkers:   10,
		Name:         "test-queue",
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

	err = queue.Push(&test1)
	assert.NoError(t, err)

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
		return
	}

	select {
	case result2 = <-handleChan:
	case <-time.After(500 * time.Millisecond):
		assert.Fail(t, "handler chan should contain test2")
	}

	assert.Equal(t, test2.TestString, result2.TestString)
	assert.Equal(t, test2.TestInt, result2.TestInt)

	// Set pushBack to so that the next handle will result in a Pause
	lock.Lock()
	pushBack = true
	lock.Unlock()

	// Ensure that we're still resumed
	_, resumed = pausable.IsPausedIsResumed()

	select {
	case <-resumed:
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "Queue is not resumed")
		return
	}

	// push test1
	queue.Push(&test1)

	// Now as this is handled it should pause
	paused, _ = pausable.IsPausedIsResumed()

	select {
	case <-paused:
	case <-handleChan:
		assert.Fail(t, "handler chan should not contain test1")
		return
	case <-time.After(500 * time.Millisecond):
		assert.Fail(t, "queue should be paused")
		return
	}

	lock.Lock()
	pushBack = false
	lock.Unlock()

	pausable.Resume()

	_, resumed = pausable.IsPausedIsResumed()
	select {
	case <-resumed:
	case <-time.After(500 * time.Millisecond):
		assert.Fail(t, "Queue should be resumed")
		return
	}

	select {
	case result1 = <-handleChan:
	case <-time.After(500 * time.Millisecond):
		assert.Fail(t, "handler chan should contain test1")
		return
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

	// Wait til it is closed
	select {
	case <-queue.(*PersistableChannelQueue).closed:
	case <-time.After(5 * time.Second):
		assert.Fail(t, "queue should close")
		return
	}

	err = queue.Push(&test1)
	assert.NoError(t, err)
	err = queue.Push(&test2)
	assert.NoError(t, err)
	select {
	case <-handleChan:
		assert.Fail(t, "Handler processing should have stopped")
		return
	default:
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
	case <-handleChan:
		assert.Fail(t, "Handler processing should have stopped")
		return
	case <-terminated:
	case <-time.After(10 * time.Second):
		assert.Fail(t, "Queue should have terminated")
		return
	}

	lock.Lock()
	pushBack = true
	lock.Unlock()

	// Reopen queue
	terminated = make(chan struct{})
	queue, err = NewPersistableChannelQueue(handle, PersistableChannelQueueConfiguration{
		DataDir:      tmpDir,
		BatchLength:  1,
		QueueLength:  20,
		Workers:      1,
		BoostWorkers: 0,
		MaxWorkers:   10,
		Name:         "test-queue",
	}, &testData{})
	assert.NoError(t, err)
	pausable, ok = queue.(Pausable)
	if !assert.True(t, ok) {
		return
	}

	paused, _ = pausable.IsPausedIsResumed()

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

	select {
	case <-handleChan:
		assert.Fail(t, "Handler processing should have stopped")
		return
	case <-paused:
	}

	paused, _ = pausable.IsPausedIsResumed()

	select {
	case <-paused:
	case <-time.After(500 * time.Millisecond):
		assert.Fail(t, "Queue is not paused")
		return
	}

	select {
	case <-handleChan:
		assert.Fail(t, "Handler processing should have stopped")
		return
	default:
	}

	lock.Lock()
	pushBack = false
	lock.Unlock()

	pausable.Resume()
	_, resumed = pausable.IsPausedIsResumed()
	select {
	case <-resumed:
	case <-time.After(500 * time.Millisecond):
		assert.Fail(t, "Queue should be resumed")
		return
	}

	var result3, result4 *testData

	select {
	case result3 = <-handleChan:
	case <-time.After(1 * time.Second):
		assert.Fail(t, "Handler processing should have resumed")
		return
	}
	select {
	case result4 = <-handleChan:
	case <-time.After(1 * time.Second):
		assert.Fail(t, "Handler processing should have resumed")
		return
	}
	if result4.TestString == test1.TestString {
		result3, result4 = result4, result3
	}
	assert.Equal(t, test1.TestString, result3.TestString)
	assert.Equal(t, test1.TestInt, result3.TestInt)

	assert.Equal(t, test2.TestString, result4.TestString)
	assert.Equal(t, test2.TestInt, result4.TestInt)

	lock.Lock()
	callbacks = make([]func(), len(queueShutdown))
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
	case <-time.After(10 * time.Second):
		assert.Fail(t, "Queue should have terminated")
		return
	case <-terminated:
	}
}
