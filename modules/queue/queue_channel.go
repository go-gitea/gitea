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
	shutdownCtx        context.Context
	shutdownCtxCancel  context.CancelFunc
	terminateCtx       context.Context
	terminateCtxCancel context.CancelFunc
	exemplar           interface{}
	workers            int
	name               string
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

	terminateCtx, terminateCtxCancel := context.WithCancel(context.Background())
	shutdownCtx, shutdownCtxCancel := context.WithCancel(terminateCtx)

	queue := &ChannelQueue{
		WorkerPool:         NewWorkerPool(handle, config.WorkerPoolConfiguration),
		shutdownCtx:        shutdownCtx,
		shutdownCtxCancel:  shutdownCtxCancel,
		terminateCtx:       terminateCtx,
		terminateCtxCancel: terminateCtxCancel,
		exemplar:           exemplar,
		workers:            config.Workers,
		name:               config.Name,
	}
	queue.qid = GetManager().Add(queue, ChannelQueueType, config, exemplar)
	return queue, nil
}

// Run starts to run the queue
func (q *ChannelQueue) Run(atShutdown, atTerminate func(func())) {
	atShutdown(q.Shutdown)
	atTerminate(q.Terminate)
	log.Debug("ChannelQueue: %s Starting", q.name)
	_ = q.AddWorkers(q.workers, 0)
}

// Push will push data into the queue
func (q *ChannelQueue) Push(data Data) error {
	if !assignableTo(data, q.exemplar) {
		return fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in queue: %s", data, q.exemplar, q.name)
	}
	q.WorkerPool.Push(data)
	return nil
}

// Shutdown processing from this queue
func (q *ChannelQueue) Shutdown() {
	q.lock.Lock()
	defer q.lock.Unlock()
	select {
	case <-q.shutdownCtx.Done():
		log.Trace("ChannelQueue: %s Already Shutting down", q.name)
		return
	default:
	}
	log.Trace("ChannelQueue: %s Shutting down", q.name)
	go func() {
		log.Trace("ChannelQueue: %s Flushing", q.name)
		if err := q.FlushWithContext(q.terminateCtx); err != nil {
			log.Warn("ChannelQueue: %s Terminated before completed flushing", q.name)
			return
		}
		log.Debug("ChannelQueue: %s Flushed", q.name)
	}()
	q.shutdownCtxCancel()
	log.Debug("ChannelQueue: %s Shutdown", q.name)
}

// Terminate this queue and close the queue
func (q *ChannelQueue) Terminate() {
	log.Trace("ChannelQueue: %s Terminating", q.name)
	q.Shutdown()
	select {
	case <-q.terminateCtx.Done():
		return
	default:
	}
	q.terminateCtxCancel()
	log.Debug("ChannelQueue: %s Terminated", q.name)
}

// Name returns the name of this queue
func (q *ChannelQueue) Name() string {
	return q.name
}

func init() {
	queuesMap[ChannelQueueType] = NewChannelQueue
}
