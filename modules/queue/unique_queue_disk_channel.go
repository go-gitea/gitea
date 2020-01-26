// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// PersistableChannelUniqueQueueType is the type for persistable queue
const PersistableChannelUniqueQueueType Type = "unique-persistable-channel"

// PersistableChannelUniqueQueueConfiguration is the configuration for a PersistableChannelUniqueQueue
type PersistableChannelUniqueQueueConfiguration struct {
	Name         string
	DataDir      string
	BatchLength  int
	QueueLength  int
	Timeout      time.Duration
	MaxAttempts  int
	Workers      int
	MaxWorkers   int
	BlockTimeout time.Duration
	BoostTimeout time.Duration
	BoostWorkers int
}

// PersistableChannelUniqueQueue wraps a channel queue and level queue together
type PersistableChannelUniqueQueue struct {
	*ChannelUniqueQueue
	delayedStarter
	lock   sync.Mutex
	closed chan struct{}
}

// NewPersistableChannelUniqueQueue creates a wrapped batched channel queue with persistable level queue backend when shutting down
// This differs from a wrapped queue in that the persistent queue is only used to persist at shutdown/terminate
func NewPersistableChannelUniqueQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(PersistableChannelUniqueQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(PersistableChannelUniqueQueueConfiguration)

	channelUniqueQueue, err := NewChannelUniqueQueue(handle, ChannelUniqueQueueConfiguration{
		WorkerPoolConfiguration: WorkerPoolConfiguration{
			QueueLength:  config.QueueLength,
			BatchLength:  config.BatchLength,
			BlockTimeout: config.BlockTimeout,
			BoostTimeout: config.BoostTimeout,
			BoostWorkers: config.BoostWorkers,
			MaxWorkers:   config.MaxWorkers,
		},
		Workers: config.Workers,
		Name:    config.Name + "-channel",
	}, exemplar)
	if err != nil {
		return nil, err
	}

	// the level backend only needs temporary workers to catch up with the previously dropped work
	levelCfg := LevelUniqueQueueConfiguration{
		WorkerPoolConfiguration: WorkerPoolConfiguration{
			QueueLength:  config.QueueLength,
			BatchLength:  config.BatchLength,
			BlockTimeout: 0,
			BoostTimeout: 0,
			BoostWorkers: 0,
			MaxWorkers:   1,
		},
		DataDir: config.DataDir,
		Workers: 1,
		Name:    config.Name + "-level",
	}

	queue := &PersistableChannelUniqueQueue{
		ChannelUniqueQueue: channelUniqueQueue.(*ChannelUniqueQueue),
		closed:             make(chan struct{}),
	}

	levelQueue, err := NewLevelUniqueQueue(func(data ...Data) {
		for _, datum := range data {
			err := queue.Push(datum)
			if err != nil && err != ErrAlreadyInQueue {
				log.Error("Unable push to channelled queue: %v", err)
			}
		}
	}, levelCfg, exemplar)
	if err == nil {
		queue.delayedStarter = delayedStarter{
			internal: levelQueue.(*LevelUniqueQueue),
			name:     config.Name,
		}

		_ = GetManager().Add(queue, PersistableChannelUniqueQueueType, config, exemplar)
		return queue, nil
	}
	if IsErrInvalidConfiguration(err) {
		// Retrying ain't gonna make this any better...
		return nil, ErrInvalidConfiguration{cfg: cfg}
	}

	queue.delayedStarter = delayedStarter{
		cfg:         levelCfg,
		underlying:  LevelUniqueQueueType,
		timeout:     config.Timeout,
		maxAttempts: config.MaxAttempts,
		name:        config.Name,
	}
	_ = GetManager().Add(queue, PersistableChannelUniqueQueueType, config, exemplar)
	return queue, nil
}

// Name returns the name of this queue
func (p *PersistableChannelUniqueQueue) Name() string {
	return p.delayedStarter.name
}

// Push will push the indexer data to queue
func (p *PersistableChannelUniqueQueue) Push(data Data) error {
	return p.PushFunc(data, nil)
}

// PushFunc will push the indexer data to queue
func (p *PersistableChannelUniqueQueue) PushFunc(data Data, fn func() error) error {
	select {
	case <-p.closed:
		return p.internal.(UniqueQueue).PushFunc(data, fn)
	default:
		return p.ChannelUniqueQueue.PushFunc(data, fn)
	}
}

// Has will test if the queue has the data
func (p *PersistableChannelUniqueQueue) Has(data Data) (bool, error) {
	// This is more difficult...
	has, err := p.ChannelUniqueQueue.Has(data)
	if err != nil || has {
		return has, err
	}
	return p.internal.(UniqueQueue).Has(data)
}

// Run starts to run the queue
func (p *PersistableChannelUniqueQueue) Run(atShutdown, atTerminate func(context.Context, func())) {
	log.Debug("PersistableChannelUniqueQueue: %s Starting", p.delayedStarter.name)

	p.lock.Lock()
	if p.internal == nil {
		err := p.setInternal(atShutdown, func(data ...Data) {
			for _, datum := range data {
				err := p.Push(datum)
				if err != nil && err != ErrAlreadyInQueue {
					log.Error("Unable push to channelled queue: %v", err)
				}
			}
		}, p.exemplar)
		p.lock.Unlock()
		if err != nil {
			log.Fatal("Unable to create internal queue for %s Error: %v", p.Name(), err)
			return
		}
	} else {
		p.lock.Unlock()
	}
	atShutdown(context.Background(), p.Shutdown)
	atTerminate(context.Background(), p.Terminate)

	// Just run the level queue - we shut it down later
	go p.internal.Run(func(_ context.Context, _ func()) {}, func(_ context.Context, _ func()) {})

	go func() {
		_ = p.ChannelUniqueQueue.AddWorkers(p.workers, 0)
	}()

	log.Trace("PersistableChannelUniqueQueue: %s Waiting til closed", p.delayedStarter.name)
	<-p.closed
	log.Trace("PersistableChannelUniqueQueue: %s Cancelling pools", p.delayedStarter.name)
	p.internal.(*LevelUniqueQueue).cancel()
	p.ChannelUniqueQueue.cancel()
	log.Trace("PersistableChannelUniqueQueue: %s Waiting til done", p.delayedStarter.name)
	p.ChannelUniqueQueue.Wait()
	p.internal.(*LevelUniqueQueue).Wait()
	// Redirect all remaining data in the chan to the internal channel
	go func() {
		log.Trace("PersistableChannelUniqueQueue: %s Redirecting remaining data", p.delayedStarter.name)
		for data := range p.ChannelUniqueQueue.dataChan {
			_ = p.internal.Push(data)
		}
		log.Trace("PersistableChannelUniqueQueue: %s Done Redirecting remaining data", p.delayedStarter.name)
	}()
	log.Trace("PersistableChannelUniqueQueue: %s Done main loop", p.delayedStarter.name)
}

// Flush flushes the queue
func (p *PersistableChannelUniqueQueue) Flush(timeout time.Duration) error {
	return p.ChannelUniqueQueue.Flush(timeout)
}

// Shutdown processing this queue
func (p *PersistableChannelUniqueQueue) Shutdown() {
	log.Trace("PersistableChannelUniqueQueue: %s Shutting down", p.delayedStarter.name)
	select {
	case <-p.closed:
	default:
		p.lock.Lock()
		defer p.lock.Unlock()
		if p.internal != nil {
			p.internal.(*LevelUniqueQueue).Shutdown()
		}
		close(p.closed)
	}
	log.Debug("PersistableChannelUniqueQueue: %s Shutdown", p.delayedStarter.name)
}

// Terminate this queue and close the queue
func (p *PersistableChannelUniqueQueue) Terminate() {
	log.Trace("PersistableChannelUniqueQueue: %s Terminating", p.delayedStarter.name)
	p.Shutdown()
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.internal != nil {
		p.internal.(*LevelUniqueQueue).Terminate()
	}
	log.Debug("PersistableChannelUniqueQueue: %s Terminated", p.delayedStarter.name)
}

func init() {
	queuesMap[PersistableChannelUniqueQueueType] = NewPersistableChannelUniqueQueue
}
