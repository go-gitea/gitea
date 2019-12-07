// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// ChannelQueueType is the type for channel queue
const ChannelQueueType Type = "channel"

// ChannelQueueConfiguration is the configuration for a ChannelQueue
type ChannelQueueConfiguration struct {
	QueueLength  int
	BatchLength  int
	Workers      int
	BlockTimeout time.Duration
	BoostTimeout time.Duration
	BoostWorkers int
}

// ChannelQueue implements
type ChannelQueue struct {
	pool     *WorkerPool
	exemplar interface{}
	workers  int
}

// NewChannelQueue create a memory channel queue
func NewChannelQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(ChannelQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(ChannelQueueConfiguration)
	if config.BatchLength == 0 {
		config.BatchLength = 1
	}
	dataChan := make(chan Data, config.QueueLength)

	ctx, cancel := context.WithCancel(context.Background())
	return &ChannelQueue{
		pool: &WorkerPool{
			baseCtx:      ctx,
			cancel:       cancel,
			batchLength:  config.BatchLength,
			handle:       handle,
			dataChan:     dataChan,
			blockTimeout: config.BlockTimeout,
			boostTimeout: config.BoostTimeout,
			boostWorkers: config.BoostWorkers,
		},
		exemplar: exemplar,
		workers:  config.Workers,
	}, nil
}

// Run starts to run the queue
func (c *ChannelQueue) Run(atShutdown, atTerminate func(context.Context, func())) {
	atShutdown(context.Background(), func() {
		log.Warn("ChannelQueue is not shutdownable!")
	})
	atTerminate(context.Background(), func() {
		log.Warn("ChannelQueue is not terminatable!")
	})
	c.pool.addWorkers(c.pool.baseCtx, c.workers)
}

// Push will push the indexer data to queue
func (c *ChannelQueue) Push(data Data) error {
	if c.exemplar != nil {
		// Assert data is of same type as r.exemplar
		t := reflect.TypeOf(data)
		exemplarType := reflect.TypeOf(c.exemplar)
		if !t.AssignableTo(exemplarType) || data == nil {
			return fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in queue: %s", data, c.exemplar, c.name)
		}
	}
	c.pool.Push(data)
	return nil
}

func init() {
	queuesMap[ChannelQueueType] = NewChannelQueue
}
