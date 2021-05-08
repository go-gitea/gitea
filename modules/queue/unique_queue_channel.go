// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"fmt"
	"sync"

	"code.gitea.io/gitea/modules/log"
)

// ChannelUniqueQueueType is the type for channel queue
const ChannelUniqueQueueType Type = "unique-channel"

// ChannelUniqueQueueConfiguration is the configuration for a ChannelUniqueQueue
type ChannelUniqueQueueConfiguration ChannelQueueConfiguration

// ChannelUniqueQueue implements UniqueQueue
//
// It is basically a thin wrapper around a WorkerPool but keeps a store of
// what has been pushed within a table.
//
// Please note that this Queue does not guarantee that a particular
// task cannot be processed twice or more at the same time. Uniqueness is
// only guaranteed whilst the task is waiting in the queue.
type ChannelUniqueQueue struct {
	*WorkerPool
	lock               sync.Mutex
	table              map[Data]bool
	shutdownCtx        context.Context
	shutdownCtxCancel  context.CancelFunc
	terminateCtx       context.Context
	terminateCtxCancel context.CancelFunc
	exemplar           interface{}
	workers            int
	name               string
}

// NewChannelUniqueQueue create a memory channel queue
func NewChannelUniqueQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(ChannelUniqueQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(ChannelUniqueQueueConfiguration)
	if config.BatchLength == 0 {
		config.BatchLength = 1
	}

	terminateCtx, terminateCtxCancel := context.WithCancel(context.Background())
	shutdownCtx, shutdownCtxCancel := context.WithCancel(terminateCtx)

	queue := &ChannelUniqueQueue{
		table:              map[Data]bool{},
		shutdownCtx:        shutdownCtx,
		shutdownCtxCancel:  shutdownCtxCancel,
		terminateCtx:       terminateCtx,
		terminateCtxCancel: terminateCtxCancel,
		exemplar:           exemplar,
		workers:            config.Workers,
		name:               config.Name,
	}
	queue.WorkerPool = NewWorkerPool(func(data ...Data) {
		for _, datum := range data {
			queue.lock.Lock()
			delete(queue.table, datum)
			queue.lock.Unlock()
			handle(datum)
		}
	}, config.WorkerPoolConfiguration)

	queue.qid = GetManager().Add(queue, ChannelUniqueQueueType, config, exemplar)
	return queue, nil
}

// Run starts to run the queue
func (q *ChannelUniqueQueue) Run(atShutdown, atTerminate func(func())) {
	atShutdown(q.Shutdown)
	atTerminate(q.Terminate)
	log.Debug("ChannelUniqueQueue: %s Starting", q.name)
	_ = q.AddWorkers(q.workers, 0)
}

// Push will push data into the queue if the data is not already in the queue
func (q *ChannelUniqueQueue) Push(data Data) error {
	return q.PushFunc(data, nil)
}

// PushFunc will push data into the queue
func (q *ChannelUniqueQueue) PushFunc(data Data, fn func() error) error {
	if !assignableTo(data, q.exemplar) {
		return fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in queue: %s", data, q.exemplar, q.name)
	}
	q.lock.Lock()
	locked := true
	defer func() {
		if locked {
			q.lock.Unlock()
		}
	}()
	if _, ok := q.table[data]; ok {
		return ErrAlreadyInQueue
	}
	// FIXME: We probably need to implement some sort of limit here
	// If the downstream queue blocks this table will grow without limit
	q.table[data] = true
	if fn != nil {
		err := fn()
		if err != nil {
			delete(q.table, data)
			return err
		}
	}
	locked = false
	q.lock.Unlock()
	q.WorkerPool.Push(data)
	return nil
}

// Has checks if the data is in the queue
func (q *ChannelUniqueQueue) Has(data Data) (bool, error) {
	q.lock.Lock()
	defer q.lock.Unlock()
	_, has := q.table[data]
	return has, nil
}

// Shutdown processing from this queue
func (q *ChannelUniqueQueue) Shutdown() {
	log.Trace("ChannelUniqueQueue: %s Shutting down", q.name)
	select {
	case <-q.shutdownCtx.Done():
		return
	default:
	}
	go func() {
		log.Trace("ChannelUniqueQueue: %s Flushing", q.name)
		if err := q.FlushWithContext(q.terminateCtx); err != nil {
			log.Warn("ChannelUniqueQueue: %s Terminated before completed flushing", q.name)
			return
		}
		log.Debug("ChannelUniqueQueue: %s Flushed", q.name)
	}()
	q.shutdownCtxCancel()
	log.Debug("ChannelUniqueQueue: %s Shutdown", q.name)
}

// Terminate this queue and close the queue
func (q *ChannelUniqueQueue) Terminate() {
	log.Trace("ChannelUniqueQueue: %s Terminating", q.name)
	q.Shutdown()
	select {
	case <-q.terminateCtx.Done():
		return
	default:
	}
	q.terminateCtxCancel()
	log.Debug("ChannelUniqueQueue: %s Terminated", q.name)
}

// Name returns the name of this queue
func (q *ChannelUniqueQueue) Name() string {
	return q.name
}

func init() {
	queuesMap[ChannelUniqueQueueType] = NewChannelUniqueQueue
}
