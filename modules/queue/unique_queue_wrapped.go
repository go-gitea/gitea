// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// WrappedUniqueQueueType is the type for a wrapped delayed starting queue
const WrappedUniqueQueueType Type = "unique-wrapped"

// WrappedUniqueQueueConfiguration is the configuration for a WrappedUniqueQueue
type WrappedUniqueQueueConfiguration struct {
	Underlying  Type
	Timeout     time.Duration
	MaxAttempts int
	Config      interface{}
	QueueLength int
	Name        string
}

// WrappedUniqueQueue wraps a delayed starting unique queue
type WrappedUniqueQueue struct {
	delayedStarter
	lock     sync.Mutex
	handle   HandlerFunc
	exemplar interface{}
	channel  chan Data
	table    map[Data]bool
	tlock    sync.Mutex
	ready    bool
}

// NewWrappedUniqueQueue will attempt to create a unique queue of the provided type,
// but if there is a problem creating this queue it will instead create
// a WrappedUniqueQueue with delayed startup of the queue instead and a
// channel which will be redirected to the queue
func NewWrappedUniqueQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(WrappedUniqueQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(WrappedUniqueQueueConfiguration)

	queue, err := NewQueue(config.Underlying, handle, config.Config, exemplar)
	if err == nil {
		// Just return the queue there is no need to wrap
		return queue, nil
	}
	if IsErrInvalidConfiguration(err) {
		// Retrying ain't gonna make this any better...
		return nil, ErrInvalidConfiguration{cfg: cfg}
	}

	wrapped := &WrappedUniqueQueue{
		channel:  make(chan Data, config.QueueLength),
		exemplar: exemplar,
		delayedStarter: delayedStarter{
			cfg:         config.Config,
			underlying:  config.Underlying,
			timeout:     config.Timeout,
			maxAttempts: config.MaxAttempts,
			name:        config.Name,
		},
		table: map[Data]bool{},
	}

	wrapped.handle = func(data ...Data) {
		for _, datum := range data {
			wrapped.tlock.Lock()
			if !wrapped.ready {
				wrapped.tlock.Lock()
				delete(wrapped.table, data)
				if len(wrapped.table) == 0 {
					wrapped.ready = true
				}
			}
			wrapped.tlock.Unlock()
			handle(datum)
		}
	}
	_ = GetManager().Add(queue, WrappedUniqueQueueType, config, exemplar)
	return wrapped, nil
}

// Name returns the name of the queue
func (q *WrappedUniqueQueue) Name() string {
	return q.name + "-wrapper"
}

// Push will push the data to the internal channel checking it against the exemplar
func (q *WrappedUniqueQueue) Push(data Data) error {
	return q.PushFunc(data, nil)
}

// PushFunc will push the data to the internal channel checking it against the exemplar
func (q *WrappedUniqueQueue) PushFunc(data Data, fn func() error) error {
	q.tlock.Lock()
	if q.ready {
		q.tlock.Unlock()
		return q.internal.(UniqueQueue).PushFunc(data, fn)
	}
	q.tlock.Unlock()

	if !assignableTo(data, q.exemplar) {
		return fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in %s", data, q.exemplar, q.name)
	}
	q.tlock.Lock()
	if q.ready {
		q.tlock.Unlock()
		return q.internal.(UniqueQueue).PushFunc(data, fn)
	}
	locked := true
	defer func() {
		if locked {
			q.tlock.Unlock()
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
	q.tlock.Unlock()

	q.channel <- data
	return nil
}

// Has checks if the data is in the queue
func (q *WrappedUniqueQueue) Has(data Data) (bool, error) {
	q.tlock.Lock()
	defer q.tlock.Unlock()
	if q.ready {
		return q.internal.(UniqueQueue).Has(data)
	}
	_, has := q.table[data]
	return has, nil
}

func (q *WrappedUniqueQueue) flushInternalWithContext(ctx context.Context) error {
	q.lock.Lock()
	if q.internal == nil {
		q.lock.Unlock()
		return fmt.Errorf("not ready to flush wrapped queue %s yet", q.Name())
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return q.internal.FlushWithContext(ctx)
}

// Flush flushes the queue and blocks till the queue is empty
func (q *WrappedUniqueQueue) Flush(timeout time.Duration) error {
	var ctx context.Context
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()
	return q.FlushWithContext(ctx)
}

func (q *WrappedUniqueQueue) FlushWithContext(ctx context.Context) error {
	log.Trace("WrappedUniqueQueue: %s FlushWithContext", q.Name())
	errChan := make(chan error, 1)
	go func() {
		errChan <- q.flushInternalWithContext(ctx)
		close(errChan)
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		go func() {
			<-errChan
		}()
		return ctx.Err()
	}
}

// IsEmpty checks whether the queue is empty
func (q *WrappedUniqueQueue) IsEmpty() bool {
	q.tlock.Lock()
	if len(q.table) > 0 {
		q.tlock.Unlock()
		return false
	}
	if q.ready {
		q.tlock.Unlock()
		return q.internal.IsEmpty()
	}
	q.tlock.Unlock()
	return false
}

// Run starts to run the queue and attempts to create the internal queue
func (q *WrappedUniqueQueue) Run(atShutdown, atTerminate func(context.Context, func())) {
	q.lock.Lock()
	if q.internal == nil {
		err := q.setInternal(atShutdown, q.handle, q.exemplar)
		q.lock.Unlock()
		if err != nil {
			log.Fatal("Unable to set the internal queue for %s Error: %v", q.Name(), err)
			return
		}
		go func() {
			for data := range q.channel {
				_ = q.internal.Push(data)
			}
		}()
	} else {
		q.lock.Unlock()
	}

	q.internal.Run(atShutdown, atTerminate)
	log.Trace("WrappedUniqueQueue: %s Done", q.name)
}

// Shutdown this queue and stop processing
func (q *WrappedUniqueQueue) Shutdown() {
	log.Trace("WrappedUniqueQueue: %s Shutdown", q.name)
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.internal == nil {
		return
	}
	if shutdownable, ok := q.internal.(Shutdownable); ok {
		shutdownable.Shutdown()
	}
}

// Terminate this queue and close the queue
func (q *WrappedUniqueQueue) Terminate() {
	log.Trace("WrappedUniqueQueue: %s Terminating", q.name)
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.internal == nil {
		return
	}
	if shutdownable, ok := q.internal.(Shutdownable); ok {
		shutdownable.Terminate()
	}
}

func init() {
	queuesMap[WrappedUniqueQueueType] = NewWrappedUniqueQueue
}
