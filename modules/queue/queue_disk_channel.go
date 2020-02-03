// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// PersistableChannelQueueType is the type for persistable queue
const PersistableChannelQueueType Type = "persistable-channel"

// PersistableChannelQueueConfiguration is the configuration for a PersistableChannelQueue
type PersistableChannelQueueConfiguration struct {
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

// PersistableChannelQueue wraps a channel queue and level queue together
// The disk level queue will be used to store data at shutdown and terminate - and will be restored
// on start up.
type PersistableChannelQueue struct {
	channelQueue *ChannelQueue
	delayedStarter
	lock   sync.Mutex
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

	channelQueue, err := NewChannelQueue(handle, ChannelQueueConfiguration{
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
	levelCfg := LevelQueueConfiguration{
		WorkerPoolConfiguration: WorkerPoolConfiguration{
			QueueLength:  config.QueueLength,
			BatchLength:  config.BatchLength,
			BlockTimeout: 1 * time.Second,
			BoostTimeout: 5 * time.Minute,
			BoostWorkers: 5,
			MaxWorkers:   6,
		},
		DataDir: config.DataDir,
		Workers: 1,
		Name:    config.Name + "-level",
	}

	levelQueue, err := NewLevelQueue(handle, levelCfg, exemplar)
	if err == nil {
		queue := &PersistableChannelQueue{
			channelQueue: channelQueue.(*ChannelQueue),
			delayedStarter: delayedStarter{
				internal: levelQueue.(*LevelQueue),
				name:     config.Name,
			},
			closed: make(chan struct{}),
		}
		_ = GetManager().Add(queue, PersistableChannelQueueType, config, exemplar)
		return queue, nil
	}
	if IsErrInvalidConfiguration(err) {
		// Retrying ain't gonna make this any better...
		return nil, ErrInvalidConfiguration{cfg: cfg}
	}

	queue := &PersistableChannelQueue{
		channelQueue: channelQueue.(*ChannelQueue),
		delayedStarter: delayedStarter{
			cfg:         levelCfg,
			underlying:  LevelQueueType,
			timeout:     config.Timeout,
			maxAttempts: config.MaxAttempts,
			name:        config.Name,
		},
		closed: make(chan struct{}),
	}
	_ = GetManager().Add(queue, PersistableChannelQueueType, config, exemplar)
	return queue, nil
}

// Name returns the name of this queue
func (p *PersistableChannelQueue) Name() string {
	return p.delayedStarter.name
}

// Push will push the indexer data to queue
func (p *PersistableChannelQueue) Push(data Data) error {
	select {
	case <-p.closed:
		return p.internal.Push(data)
	default:
		return p.channelQueue.Push(data)
	}
}

// Run starts to run the queue
func (p *PersistableChannelQueue) Run(atShutdown, atTerminate func(context.Context, func())) {
	log.Debug("PersistableChannelQueue: %s Starting", p.delayedStarter.name)

	p.lock.Lock()
	if p.internal == nil {
		err := p.setInternal(atShutdown, p.channelQueue.handle, p.channelQueue.exemplar)
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
		_ = p.channelQueue.AddWorkers(p.channelQueue.workers, 0)
	}()

	log.Trace("PersistableChannelQueue: %s Waiting til closed", p.delayedStarter.name)
	<-p.closed
	log.Trace("PersistableChannelQueue: %s Cancelling pools", p.delayedStarter.name)
	p.channelQueue.cancel()
	p.internal.(*LevelQueue).cancel()
	log.Trace("PersistableChannelQueue: %s Waiting til done", p.delayedStarter.name)
	p.channelQueue.Wait()
	p.internal.(*LevelQueue).Wait()
	// Redirect all remaining data in the chan to the internal channel
	go func() {
		log.Trace("PersistableChannelQueue: %s Redirecting remaining data", p.delayedStarter.name)
		for data := range p.channelQueue.dataChan {
			_ = p.internal.Push(data)
			atomic.AddInt64(&p.channelQueue.numInQueue, -1)
		}
		log.Trace("PersistableChannelQueue: %s Done Redirecting remaining data", p.delayedStarter.name)
	}()
	log.Trace("PersistableChannelQueue: %s Done main loop", p.delayedStarter.name)
}

// Flush flushes the queue and blocks till the queue is empty
func (p *PersistableChannelQueue) Flush(timeout time.Duration) error {
	var ctx context.Context
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()
	return p.FlushWithContext(ctx)
}

// FlushWithContext flushes the queue and blocks till the queue is empty
func (p *PersistableChannelQueue) FlushWithContext(ctx context.Context) error {
	errChan := make(chan error, 1)
	go func() {
		errChan <- p.channelQueue.FlushWithContext(ctx)
	}()
	go func() {
		p.lock.Lock()
		if p.internal == nil {
			p.lock.Unlock()
			errChan <- fmt.Errorf("not ready to flush internal queue %s yet", p.Name())
			return
		}
		p.lock.Unlock()
		errChan <- p.internal.FlushWithContext(ctx)
	}()
	err1 := <-errChan
	err2 := <-errChan

	if err1 != nil {
		return err1
	}
	return err2
}

// IsEmpty checks if a queue is empty
func (p *PersistableChannelQueue) IsEmpty() bool {
	if !p.channelQueue.IsEmpty() {
		return false
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.internal == nil {
		return false
	}
	return p.internal.IsEmpty()
}

// Shutdown processing this queue
func (p *PersistableChannelQueue) Shutdown() {
	log.Trace("PersistableChannelQueue: %s Shutting down", p.delayedStarter.name)
	select {
	case <-p.closed:
	default:
		p.lock.Lock()
		defer p.lock.Unlock()
		if p.internal != nil {
			p.internal.(*LevelQueue).Shutdown()
		}
		close(p.closed)
	}
	log.Debug("PersistableChannelQueue: %s Shutdown", p.delayedStarter.name)
}

// Terminate this queue and close the queue
func (p *PersistableChannelQueue) Terminate() {
	log.Trace("PersistableChannelQueue: %s Terminating", p.delayedStarter.name)
	p.Shutdown()
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.internal != nil {
		p.internal.(*LevelQueue).Terminate()
	}
	log.Debug("PersistableChannelQueue: %s Terminated", p.delayedStarter.name)
}

func init() {
	queuesMap[PersistableChannelQueueType] = NewPersistableChannelQueue
}
