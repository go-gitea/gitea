// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"gitea.com/lunny/levelqueue"
)

// LevelQueueType is the type for level queue
const LevelQueueType Type = "level"

// LevelQueueConfiguration is the configuration for a LevelQueue
type LevelQueueConfiguration struct {
	ByteFIFOQueueConfiguration
	DataDir string
}

// LevelQueue implements a disk library queue
type LevelQueue struct {
	*ByteFIFOQueue
}

// NewLevelQueue creates a ledis local queue
func NewLevelQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(LevelQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(LevelQueueConfiguration)

	byteFIFO, err := NewLevelQueueByteFIFO(config.DataDir)
	if err != nil {
		return nil, err
	}

	byteFIFOQueue, err := NewByteFIFOQueue(LevelQueueType, byteFIFO, handle, config.ByteFIFOQueueConfiguration, exemplar)
	if err != nil {
		return nil, err
	}

	queue := &LevelQueue{
		ByteFIFOQueue: byteFIFOQueue,
	}
	queue.qid = GetManager().Add(queue, LevelQueueType, config, exemplar)
	return queue, nil
}

var _ (ByteFIFO) = &LevelQueueByteFIFO{}

// LevelQueueByteFIFO represents a ByteFIFO formed from a LevelQueue
type LevelQueueByteFIFO struct {
	internal *levelqueue.Queue
}

// NewLevelQueueByteFIFO creates a ByteFIFO formed from a LevelQueue
func NewLevelQueueByteFIFO(dataDir string) (*LevelQueueByteFIFO, error) {
	internal, err := levelqueue.Open(dataDir)
	if err != nil {
		return nil, err
	}

	return &LevelQueueByteFIFO{
		internal: internal,
	}, nil
}

// PushFunc will push data into the fifo
func (fifo *LevelQueueByteFIFO) PushFunc(data []byte, fn func() error) error {
	if fn != nil {
		if err := fn(); err != nil {
			return err
		}
	}
	return fifo.internal.LPush(data)
}

// Pop pops data from the start of the fifo
func (fifo *LevelQueueByteFIFO) Pop() ([]byte, error) {
	data, err := fifo.internal.RPop()
	if err != nil && err != levelqueue.ErrNotFound {
		return nil, err
	}
	return data, nil
}

// Close this fifo
func (fifo *LevelQueueByteFIFO) Close() error {
	return fifo.internal.Close()
}

// Len returns the length of the fifo
func (fifo *LevelQueueByteFIFO) Len() int64 {
	return fifo.internal.Len()
}

func init() {
	queuesMap[LevelQueueType] = NewLevelQueue
}
