// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"sync"
	"time"
)

// PersistableChannelQueueType is the type for persistable queue
const PersistableChannelQueueType Type = "persistable-channel"

// PersistableChannelQueueConfiguration is the configuration for a PersistableChannelQueue
type PersistableChannelQueueConfiguration struct {
	DataDir     string
	BatchLength int
	QueueLength int
	Timeout     time.Duration
	MaxAttempts int
	Workers     int
}

// PersistableChannelQueue wraps a channel queue and level queue together
type PersistableChannelQueue struct {
	*BatchedChannelQueue
	delayedStarter
	closed chan struct{}
}

// NewPersistableChannelQueue creates a wrapped batched channel queue with persistable level queue backend when shutting down
// This differs from a wrapped queue in that the persistent queue is only used to persist at shutdown/terminate
func NewPersistableChannelQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(PersistableChannelQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(PersistableChannelQueueConfiguration)

	batchChannelQueue, err := NewBatchedChannelQueue(handle, BatchedChannelQueueConfiguration{
		QueueLength: config.QueueLength,
		BatchLength: config.BatchLength,
		Workers:     config.Workers,
	}, exemplar)
	if err != nil {
		return nil, err
	}

	// the level backend only needs one worker to catch up with the previously dropped work
	levelCfg := LevelQueueConfiguration{
		DataDir:     config.DataDir,
		BatchLength: config.BatchLength,
		Workers:     1,
	}

	levelQueue, err := NewLevelQueue(handle, levelCfg, exemplar)
	if err == nil {
		return &PersistableChannelQueue{
			BatchedChannelQueue: batchChannelQueue.(*BatchedChannelQueue),
			delayedStarter: delayedStarter{
				internal: levelQueue.(*LevelQueue),
			},
			closed: make(chan struct{}),
		}, nil
	}
	if IsErrInvalidConfiguration(err) {
		// Retrying ain't gonna make this any better...
		return nil, ErrInvalidConfiguration{cfg: cfg}
	}

	return &PersistableChannelQueue{
		BatchedChannelQueue: batchChannelQueue.(*BatchedChannelQueue),
		delayedStarter: delayedStarter{
			cfg:         levelCfg,
			underlying:  LevelQueueType,
			timeout:     config.Timeout,
			maxAttempts: config.MaxAttempts,
		},
		closed: make(chan struct{}),
	}, nil
}

// Push will push the indexer data to queue
func (p *PersistableChannelQueue) Push(data Data) error {
	select {
	case <-p.closed:
		return p.internal.Push(data)
	default:
		return p.BatchedChannelQueue.Push(data)
	}
}

// Run starts to run the queue
func (p *PersistableChannelQueue) Run(atShutdown, atTerminate func(context.Context, func())) {
	p.lock.Lock()
	if p.internal == nil {
		p.setInternal(atShutdown, p.handle, p.exemplar)
	} else {
		p.lock.Unlock()
	}
	atShutdown(context.Background(), p.Shutdown)
	atTerminate(context.Background(), p.Terminate)

	// Just run the level queue - we shut it down later
	go p.internal.Run(func(_ context.Context, _ func()) {}, func(_ context.Context, _ func()) {})

	wg := sync.WaitGroup{}
	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go func() {
			p.worker()
			wg.Done()
		}()
	}
	wg.Wait()
}

func (p *PersistableChannelQueue) worker() {
	delay := time.Millisecond * 300
	var datas = make([]Data, 0, p.batchLength)
loop:
	for {
		select {
		case data := <-p.queue:
			datas = append(datas, data)
			if len(datas) >= p.batchLength {
				p.handle(datas...)
				datas = make([]Data, 0, p.batchLength)
			}
		case <-time.After(delay):
			delay = time.Millisecond * 100
			if len(datas) > 0 {
				p.handle(datas...)
				datas = make([]Data, 0, p.batchLength)
			}
		case <-p.closed:
			if len(datas) > 0 {
				p.handle(datas...)
			}
			break loop
		}
	}
	go func() {
		for data := range p.queue {
			_ = p.internal.Push(data)
		}
	}()
}

// Shutdown processing this queue
func (p *PersistableChannelQueue) Shutdown() {
	select {
	case <-p.closed:
	default:
		close(p.closed)
		p.lock.Lock()
		defer p.lock.Unlock()
		if p.internal != nil {
			p.internal.(*LevelQueue).Shutdown()
		}
	}
}

// Terminate this queue and close the queue
func (p *PersistableChannelQueue) Terminate() {
	p.Shutdown()
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.internal != nil {
		p.internal.(*LevelQueue).Terminate()
	}
}

func init() {
	queuesMap[PersistableChannelQueueType] = NewPersistableChannelQueue
}
