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
	lock     sync.Mutex
	table    map[Data]bool
	exemplar interface{}
	workers  int
	name     string
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
	queue := &ChannelUniqueQueue{
		table:    map[Data]bool{},
		exemplar: exemplar,
		workers:  config.Workers,
		name:     config.Name,
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
func (q *ChannelUniqueQueue) Run(atShutdown, atTerminate func(context.Context, func())) {
	atShutdown(context.Background(), func() {
		log.Warn("ChannelUniqueQueue: %s is not shutdownable!", q.name)
	})
	atTerminate(context.Background(), func() {
		log.Warn("ChannelUniqueQueue: %s is not terminatable!", q.name)
	})
	log.Debug("ChannelUniqueQueue: %s Starting", q.name)
	go func() {
		_ = q.AddWorkers(q.workers, 0)
	}()
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

// Name returns the name of this queue
func (q *ChannelUniqueQueue) Name() string {
	return q.name
}

func init() {
	queuesMap[ChannelUniqueQueueType] = NewChannelUniqueQueue
}
