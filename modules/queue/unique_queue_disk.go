// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"

	"gitea.com/lunny/levelqueue"
)

// LevelUniqueQueueType is the type for level queue
const LevelUniqueQueueType Type = "unique-level"

// LevelUniqueQueueConfiguration is the configuration for a LevelUniqueQueue
type LevelUniqueQueueConfiguration struct {
	WorkerPoolConfiguration
	DataDir string
	Workers int
	Name    string
}

// LevelUniqueQueue implements a disk library queue
type LevelUniqueQueue struct {
	*WorkerPool
	queue      *levelqueue.UniqueQueue
	closed     chan struct{}
	terminated chan struct{}
	lock       sync.Mutex
	exemplar   interface{}
	workers    int
	name       string
}

// NewLevelUniqueQueue creates a ledis local queue
func NewLevelUniqueQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(LevelUniqueQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(LevelUniqueQueueConfiguration)

	internal, err := levelqueue.OpenUnique(config.DataDir)
	if err != nil {
		return nil, err
	}

	queue := &LevelUniqueQueue{
		WorkerPool: NewWorkerPool(handle, config.WorkerPoolConfiguration),
		queue:      internal,
		exemplar:   exemplar,
		closed:     make(chan struct{}),
		terminated: make(chan struct{}),
		workers:    config.Workers,
		name:       config.Name,
	}
	queue.qid = GetManager().Add(queue, LevelUniqueQueueType, config, exemplar)
	return queue, nil
}

// Run starts to run the queue
func (l *LevelUniqueQueue) Run(atShutdown, atTerminate func(context.Context, func())) {
	atShutdown(context.Background(), l.Shutdown)
	atTerminate(context.Background(), l.Terminate)
	log.Debug("LevelUniqueQueue: %s Starting", l.name)

	go func() {
		_ = l.AddWorkers(l.workers, 0)
	}()

	go l.readToChan()

	log.Trace("LevelUniqueQueue: %s Waiting til closed", l.name)
	<-l.closed

	log.Trace("LevelUniqueQueue: %s Waiting til done", l.name)
	l.Wait()

	log.Trace("LevelUniqueQueue: %s Waiting til cleaned", l.name)
	ctx, cancel := context.WithCancel(context.Background())
	atTerminate(ctx, cancel)
	l.CleanUp(ctx)
	cancel()
	log.Trace("LevelUniqueQueue: %s Cleaned", l.name)

}

func (l *LevelUniqueQueue) readToChan() {
	for {
		select {
		case <-l.closed:
			// tell the pool to shutdown.
			l.cancel()
			return
		default:
			bs, err := l.queue.RPop()
			if err != nil {
				if err != levelqueue.ErrNotFound {
					log.Error("LevelUniqueQueue: %s Error on RPop: %v", l.name, err)
				}
				time.Sleep(time.Millisecond * 100)
				continue
			}

			if len(bs) == 0 {
				time.Sleep(time.Millisecond * 100)
				continue
			}

			data, err := unmarshalAs(bs, l.exemplar)
			if err != nil {
				log.Error("LevelUniqueQueue: %s Failed to unmarshal with error: %v", l.name, err)
				time.Sleep(time.Millisecond * 100)
				continue
			}

			log.Trace("LevelUniqueQueue %s: Task found: %#v", l.name, data)
			l.WorkerPool.Push(data)

		}
	}
}

// Push will push the data to the queue
func (l *LevelUniqueQueue) Push(data Data) error {
	return l.PushFunc(data, nil)
}

// PushFunc will push the data to the queue running fn if the data will be added
func (l *LevelUniqueQueue) PushFunc(data Data, fn func() error) error {
	if !assignableTo(data, l.exemplar) {
		return fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in %s", data, l.exemplar, l.name)
	}
	bs, err := json.Marshal(data)
	if err != nil {
		return err
	}
	err = l.queue.LPushFunc(bs, fn)
	if err == levelqueue.ErrAlreadyInQueue {
		return ErrAlreadyInQueue
	}
	return err
}

// Has checks if the provided data is in the queue already
func (l *LevelUniqueQueue) Has(data Data) (bool, error) {
	if !assignableTo(data, l.exemplar) {
		return false, fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in %s", data, l.exemplar, l.name)
	}
	bs, err := json.Marshal(data)
	if err != nil {
		return false, err
	}
	return l.queue.Has(bs)
}

// Shutdown this queue and stop processing
func (l *LevelUniqueQueue) Shutdown() {
	l.lock.Lock()
	defer l.lock.Unlock()
	log.Trace("LevelUniqueQueue: %s Shutting down", l.name)
	select {
	case <-l.closed:
	default:
		close(l.closed)
	}
	log.Debug("LevelUniqueQueue: %s Shutdown", l.name)

}

// Terminate this queue and close the queue
func (l *LevelUniqueQueue) Terminate() {
	log.Trace("LevelUniqueQueue: %s Terminating", l.name)
	l.Shutdown()
	l.lock.Lock()
	select {
	case <-l.terminated:
		l.lock.Unlock()
	default:
		close(l.terminated)
		l.lock.Unlock()
		if err := l.queue.Close(); err != nil && err.Error() != "leveldb: closed" {
			log.Error("Error whilst closing internal queue in %s: %v", l.name, err)
		}
	}
	log.Debug("LevelUniqueQueue: %s Terminated", l.name)
}

// Name returns the name of this queue
func (l *LevelUniqueQueue) Name() string {
	return l.name
}

func init() {
	queuesMap[LevelUniqueQueueType] = NewLevelUniqueQueue
}
