// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"fmt"
	"reflect"

	"code.gitea.io/gitea/modules/log"
)

// ChannelQueueType is the type for channel queue
const ChannelQueueType Type = "channel"

// ChannelQueueConfiguration is the configuration for a ChannelQueue
type ChannelQueueConfiguration struct {
	QueueLength int
}

// ChannelQueue implements
type ChannelQueue struct {
	queue    chan Data
	handle   HandlerFunc
	exemplar interface{}
}

// NewChannelQueue create a memory channel queue
func NewChannelQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(ChannelQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(ChannelQueueConfiguration)
	return &ChannelQueue{
		queue:    make(chan Data, config.QueueLength),
		handle:   handle,
		exemplar: exemplar,
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
	go func() {
		for data := range c.queue {
			c.handle(data)
		}
	}()
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
	c.queue <- data
	return nil
}

func init() {
	queuesMap[ChannelQueueType] = NewChannelQueue
}
