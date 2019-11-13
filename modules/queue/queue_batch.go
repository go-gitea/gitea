// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// BatchedChannelQueueType is the type for batched channel queue
const BatchedChannelQueueType Type = "batched-channel"

// BatchedChannelQueueConfiguration is the configuration for a BatchedChannelQueue
type BatchedChannelQueueConfiguration struct {
	QueueLength int
	BatchLength int
}

// BatchedChannelQueue implements
type BatchedChannelQueue struct {
	*ChannelQueue
	batchLength int
}

// NewBatchedChannelQueue create a memory channel queue
func NewBatchedChannelQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(BatchedChannelQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(BatchedChannelQueueConfiguration)
	return &BatchedChannelQueue{
		&ChannelQueue{
			queue:    make(chan Data, config.QueueLength),
			handle:   handle,
			exemplar: exemplar,
		},
		config.BatchLength,
	}, nil
}

// Run starts to run the queue
func (c *BatchedChannelQueue) Run(atShutdown, atTerminate func(context.Context, func())) {
	atShutdown(context.Background(), func() {
		log.Warn("BatchedChannelQueue is not shutdownable!")
	})
	atTerminate(context.Background(), func() {
		log.Warn("BatchedChannelQueue is not terminatable!")
	})
	go func() {
		delay := time.Millisecond * 300
		var datas = make([]Data, 0, c.batchLength)
		for {
			select {
			case data := <-c.queue:
				datas = append(datas, data)
				if len(datas) >= c.batchLength {
					c.handle(datas...)
					datas = make([]Data, 0, c.batchLength)
				}
			case <-time.After(delay):
				delay = time.Millisecond * 100
				if len(datas) > 0 {
					c.handle(datas...)
					datas = make([]Data, 0, c.batchLength)
				}
			}
		}
	}()
}

func init() {
	queuesMap[BatchedChannelQueueType] = NewBatchedChannelQueue
}
