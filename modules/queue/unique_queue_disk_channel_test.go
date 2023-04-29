// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/log"

	"github.com/stretchr/testify/assert"
)

func TestPersistableChannelUniqueQueue(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping because test is flaky on CI")
	}

	tmpDir := t.TempDir()
	_ = log.NewLogger(1000, "console", "console", `{"level":"warn","stacktracelevel":"NONE","stderr":true}`)

	// Common function to create the Queue
	newQueue := func(name string, handle func(data ...Data) []Data) Queue {
		q, err := NewPersistableChannelUniqueQueue(handle,
			PersistableChannelUniqueQueueConfiguration{
				Name:         name,
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
		chans := &channels{
			readyForShutdown:  make(chan struct{}),
			readyForTerminate: make(chan struct{}),
			signalShutdown:    make(chan struct{}),
			doneShutdown:      make(chan struct{}),
		}
		go q.Run(func(atShutdown func()) {
			go func() {
				lock.Lock()
				select {
				case <-chans.readyForShutdown:
				default:
					close(chans.readyForShutdown)
				}
				lock.Unlock()
				<-chans.signalShutdown
				atShutdown()
				close(chans.doneShutdown)
			}()
		}, func(atTerminate func()) {
			lock.Lock()
			defer lock.Unlock()
			select {
			case <-chans.readyForTerminate:
			default:
				close(chans.readyForTerminate)
			}
			chans.queueTerminate = append(chans.queueTerminate, atTerminate)
		})

		return chans
	}

	// call to shutdown and terminate the queue associated with the channels
	doTerminate := func(chans *channels, lock *sync.Mutex) {
		<-chans.readyForTerminate

		lock.Lock()
		callbacks := []func(){}
		callbacks = append(callbacks, chans.queueTerminate...)
		lock.Unlock()

		for _, callback := range callbacks {
			callback()
		}
	}

	mapLock := sync.Mutex{}
	executedInitial := map[string][]string{}
	hasInitial := map[string][]string{}

	fillQueue := func(name string, done chan struct{}) {
		t.Run("Initial Filling: "+name, func(t *testing.T) {
			lock := sync.Mutex{}

			startAt100Queued := make(chan struct{})
			stopAt20Shutdown := make(chan struct{}) // stop and shutdown at the 20th item

			handle := func(data ...Data) []Data {
				<-startAt100Queued
				for _, datum := range data {
					s := datum.(string)
					mapLock.Lock()
					executedInitial[name] = append(executedInitial[name], s)
					mapLock.Unlock()
					if s == "task-20" {
						close(stopAt20Shutdown)
					}
				}
				return nil
			}

			q := newQueue(name, handle)

			// add 100 tasks to the queue
			for i := 0; i < 100; i++ {
				_ = q.Push("task-" + strconv.Itoa(i))
			}
			close(startAt100Queued)

			chans := runQueue(q, &lock)

			<-chans.readyForShutdown
			<-stopAt20Shutdown
			close(chans.signalShutdown)
			<-chans.doneShutdown
			_ = q.Push("final")

			// check which tasks are still in the queue
			for i := 0; i < 100; i++ {
				if has, _ := q.(UniqueQueue).Has("task-" + strconv.Itoa(i)); has {
					mapLock.Lock()
					hasInitial[name] = append(hasInitial[name], "task-"+strconv.Itoa(i))
					mapLock.Unlock()
				}
			}
			if has, _ := q.(UniqueQueue).Has("final"); has {
				mapLock.Lock()
				hasInitial[name] = append(hasInitial[name], "final")
				mapLock.Unlock()
			} else {
				assert.Fail(t, "UnqueQueue %s should have \"final\"", name)
			}
			doTerminate(chans, &lock)
			mapLock.Lock()
			assert.Equal(t, 101, len(executedInitial[name])+len(hasInitial[name]))
			mapLock.Unlock()
		})
		close(done)
	}

	doneA := make(chan struct{})
	doneB := make(chan struct{})

	go fillQueue("QueueA", doneA)
	go fillQueue("QueueB", doneB)

	<-doneA
	<-doneB

	executedEmpty := map[string][]string{}
	hasEmpty := map[string][]string{}
	emptyQueue := func(name string, done chan struct{}) {
		t.Run("Empty Queue: "+name, func(t *testing.T) {
			lock := sync.Mutex{}
			stop := make(chan struct{})

			// collect the tasks that have been executed
			handle := func(data ...Data) []Data {
				lock.Lock()
				for _, datum := range data {
					mapLock.Lock()
					executedEmpty[name] = append(executedEmpty[name], datum.(string))
					mapLock.Unlock()
					if datum.(string) == "final" {
						close(stop)
					}
				}
				lock.Unlock()
				return nil
			}

			q := newQueue(name, handle)
			chans := runQueue(q, &lock)

			<-chans.readyForShutdown
			<-stop
			close(chans.signalShutdown)
			<-chans.doneShutdown

			// check which tasks are still in the queue
			for i := 0; i < 100; i++ {
				if has, _ := q.(UniqueQueue).Has("task-" + strconv.Itoa(i)); has {
					mapLock.Lock()
					hasEmpty[name] = append(hasEmpty[name], "task-"+strconv.Itoa(i))
					mapLock.Unlock()
				}
			}
			doTerminate(chans, &lock)

			mapLock.Lock()
			assert.Equal(t, 101, len(executedInitial[name])+len(executedEmpty[name]))
			assert.Empty(t, hasEmpty[name])
			mapLock.Unlock()
		})
		close(done)
	}

	doneA = make(chan struct{})
	doneB = make(chan struct{})

	go emptyQueue("QueueA", doneA)
	go emptyQueue("QueueB", doneB)

	<-doneA
	<-doneB

	mapLock.Lock()
	t.Logf("TestPersistableChannelUniqueQueue executedInitiallyA=%v, executedInitiallyB=%v, executedToEmptyA=%v, executedToEmptyB=%v",
		len(executedInitial["QueueA"]), len(executedInitial["QueueB"]), len(executedEmpty["QueueA"]), len(executedEmpty["QueueB"]))

	// reset and rerun
	executedInitial = map[string][]string{}
	hasInitial = map[string][]string{}
	executedEmpty = map[string][]string{}
	hasEmpty = map[string][]string{}
	mapLock.Unlock()

	doneA = make(chan struct{})
	doneB = make(chan struct{})

	go fillQueue("QueueA", doneA)
	go fillQueue("QueueB", doneB)

	<-doneA
	<-doneB

	doneA = make(chan struct{})
	doneB = make(chan struct{})

	go emptyQueue("QueueA", doneA)
	go emptyQueue("QueueB", doneB)

	<-doneA
	<-doneB

	mapLock.Lock()
	t.Logf("TestPersistableChannelUniqueQueue executedInitiallyA=%v, executedInitiallyB=%v, executedToEmptyA=%v, executedToEmptyB=%v",
		len(executedInitial["QueueA"]), len(executedInitial["QueueB"]), len(executedEmpty["QueueA"]), len(executedEmpty["QueueB"]))
	mapLock.Unlock()
}
