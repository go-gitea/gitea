// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/log"

	"github.com/stretchr/testify/assert"
)

func TestPersistableChannelUniqueQueue(t *testing.T) {
	tmpDir := t.TempDir()
	fmt.Printf("TempDir %s\n", tmpDir)
	_ = log.NewLogger(1000, "console", "console", `{"level":"trace","stacktracelevel":"NONE","stderr":true}`)

	// Common function to create the Queue
	newQueue := func(handle func(data ...Data) []Data) Queue {
		q, err := NewPersistableChannelUniqueQueue(handle,
			PersistableChannelUniqueQueueConfiguration{
				Name:         "TestPersistableChannelUniqueQueue",
				DataDir:      tmpDir,
				QueueLength:  200,
				MaxWorkers:   1,
				BlockTimeout: 1 * time.Second,
				BoostTimeout: 5 * time.Minute,
				BoostWorkers: 1,
				Workers:      0,
			}, "task-0")
		assert.NoError(t, err)
		return q
	}

	// runs the provided queue and provides some timer function
	type channels struct {
		readyForShutdown  chan struct{} // closed when shutdown functions have been assigned
		readyForTerminate chan struct{} // closed when terminate functions have been assigned
		signalShutdown    chan struct{} // Should close to signal shutdown
		doneShutdown      chan struct{} // closed when shutdown function is done
		queueTerminate    []func()      // list of atTerminate functions to call atTerminate - need to be accessed with lock
	}
	runQueue := func(q Queue, lock *sync.Mutex) *channels {
		returnable := &channels{
			readyForShutdown:  make(chan struct{}),
			readyForTerminate: make(chan struct{}),
			signalShutdown:    make(chan struct{}),
			doneShutdown:      make(chan struct{}),
		}
		go q.Run(func(atShutdown func()) {
			go func() {
				lock.Lock()
				select {
				case <-returnable.readyForShutdown:
				default:
					close(returnable.readyForShutdown)
				}
				lock.Unlock()
				<-returnable.signalShutdown
				atShutdown()
				close(returnable.doneShutdown)
			}()
		}, func(atTerminate func()) {
			lock.Lock()
			defer lock.Unlock()
			select {
			case <-returnable.readyForTerminate:
			default:
				close(returnable.readyForTerminate)
			}
			returnable.queueTerminate = append(returnable.queueTerminate, atTerminate)
		})

		return returnable
	}

	// call to shutdown and terminate the queue associated with the channels
	shutdownAndTerminate := func(chans *channels, lock *sync.Mutex) {
		close(chans.signalShutdown)
		<-chans.doneShutdown
		<-chans.readyForTerminate

		lock.Lock()
		callbacks := []func(){}
		callbacks = append(callbacks, chans.queueTerminate...)
		lock.Unlock()

		for _, callback := range callbacks {
			callback()
		}
	}

	executedTasks1 := []string{}
	hasTasks1 := []string{}

	t.Run("Initial Filling", func(t *testing.T) {
		lock := sync.Mutex{}

		startAt100Queued := make(chan struct{})
		stopAt20Shutdown := make(chan struct{}) // stop and shutdown at the 20th item

		handle := func(data ...Data) []Data {
			<-startAt100Queued
			for _, datum := range data {
				s := datum.(string)
				lock.Lock()
				executedTasks1 = append(executedTasks1, s)
				lock.Unlock()
				if s == "task-20" {
					close(stopAt20Shutdown)
				}
			}
			return nil
		}

		q := newQueue(handle)

		// add 100 tasks to the queue
		for i := 0; i < 100; i++ {
			_ = q.Push("task-" + strconv.Itoa(i))
		}
		close(startAt100Queued)

		chans := runQueue(q, &lock)

		<-chans.readyForShutdown
		<-stopAt20Shutdown
		shutdownAndTerminate(chans, &lock)

		// check which tasks are still in the queue
		for i := 0; i < 100; i++ {
			if has, _ := q.(UniqueQueue).Has("task-" + strconv.Itoa(i)); has {
				hasTasks1 = append(hasTasks1, "task-"+strconv.Itoa(i))
			}
		}
		assert.Equal(t, 100, len(executedTasks1)+len(hasTasks1))
	})

	executedTasks2 := []string{}
	hasTasks2 := []string{}
	t.Run("Ensure that things will empty on restart", func(t *testing.T) {
		lock := sync.Mutex{}
		stop := make(chan struct{})

		// collect the tasks that have been executed
		handle := func(data ...Data) []Data {
			lock.Lock()
			for _, datum := range data {
				t.Logf("executed %s", datum.(string))
				executedTasks2 = append(executedTasks2, datum.(string))
				if datum.(string) == "task-99" {
					close(stop)
				}
			}
			lock.Unlock()
			return nil
		}

		q := newQueue(handle)
		chans := runQueue(q, &lock)

		<-chans.readyForShutdown
		<-stop
		shutdownAndTerminate(chans, &lock)

		// check which tasks are still in the queue
		for i := 0; i < 100; i++ {
			if has, _ := q.(UniqueQueue).Has("task-" + strconv.Itoa(i)); has {
				hasTasks2 = append(hasTasks2, "task-"+strconv.Itoa(i))
			}
		}

		assert.Equal(t, 100, len(executedTasks1)+len(executedTasks2))
		assert.Equal(t, 0, len(hasTasks2))
	})

	executedTasks3 := []string{}
	hasTasks3 := []string{}

	t.Run("refill", func(t *testing.T) {
		lock := sync.Mutex{}
		stop := make(chan struct{})

		handle := func(data ...Data) []Data {
			lock.Lock()
			for _, datum := range data {
				executedTasks3 = append(executedTasks3, datum.(string))
			}
			lock.Unlock()
			return nil
		}

		q := newQueue(handle)
		chans := runQueue(q, &lock)

		// re-run all tasks
		for i := 0; i < 100; i++ {
			_ = q.Push("task-" + strconv.Itoa(i))
		}

		// wait for a while
		time.Sleep(1 * time.Second)

		close(stop)
		<-chans.readyForShutdown
		shutdownAndTerminate(chans, &lock)

		// check whether the tasks are still in the queue
		for i := 0; i < 100; i++ {
			if has, _ := q.(UniqueQueue).Has("task-" + strconv.Itoa(i)); has {
				hasTasks3 = append(hasTasks3, "task-"+strconv.Itoa(i))
			}
		}
		assert.Equal(t, 100, len(executedTasks3)+len(hasTasks3))
	})

	t.Logf("TestPersistableChannelUniqueQueue completed1=%v, executed2=%v, has2=%v, executed3=%v, has3=%v",
		len(executedTasks1), len(executedTasks2), len(hasTasks2), len(executedTasks3), len(hasTasks3))
}
