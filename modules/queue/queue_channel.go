// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/log"
)

// ChannelQueueType is the type for channel queue
const ChannelQueueType Type = "channel"

// ChannelQueueConfiguration is the configuration for a ChannelQueue
type ChannelQueueConfiguration struct {
	WorkerPoolConfiguration
	Workers int
	Name    string
}

// ChannelQueue implements Queue
//
// A channel queue is not persistable and does not shutdown or terminate cleanly
// It is basically a very thin wrapper around a WorkerPool
type ChannelQueue struct {
	*WorkerPool
	exemplar interface{}
	workers  int
	name     string
}

// NewChannelQueue creates a memory channel queue
func NewChannelQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(ChannelQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(ChannelQueueConfiguration)
	if config.BatchLength == 0 {
		config.BatchLength = 1
	}
	queue := &ChannelQueue{
		WorkerPool: NewWorkerPool(handle, config.WorkerPoolConfiguration),
		exemplar:   exemplar,
		workers:    config.Workers,
		name:       config.Name,
	}
	queue.qid = GetManager().Add(queue, ChannelQueueType, config, exemplar)
	return queue, nil
}

// Run starts to run the queue
func (q *ChannelQueue) Run(atShutdown, atTerminate func(context.Context, func())) {
	atShutdown(context.Background(), func() {
		log.Warn("ChannelQueue: %s is not shutdownable!", q.name)
	})
	atTerminate(context.Background(), func() {
		log.Warn("ChannelQueue: %s is not terminatable!", q.name)
	})
	log.Debug("ChannelQueue: %s Starting", q.name)
	go func() {
		_ = q.AddWorkers(q.workers, 0)
	}()
}

// Push will push data into the queue
func (q *ChannelQueue) Push(data Data) error {
	if !assignableTo(data, q.exemplar) {
		return fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in queue: %s", data, q.exemplar, q.name)
	}
	q.WorkerPool.Push(data)
	return nil
}

// Name returns the name of this queue
func (q *ChannelQueue) Name() string {
	return q.name
}

func init() {
	queuesMap[ChannelQueueType] = NewChannelQueue
}
