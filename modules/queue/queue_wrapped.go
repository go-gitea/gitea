// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// WrappedQueueType is the type for a wrapped delayed starting queue
const WrappedQueueType Type = "wrapped"

// WrappedQueueConfiguration is the configuration for a WrappedQueue
type WrappedQueueConfiguration struct {
	Underlying  Type
	Timeout     time.Duration
	MaxAttempts int
	Config      interface{}
	QueueLength int
	Name        string
}

type delayedStarter struct {
	lock        sync.Mutex
	internal    Queue
	underlying  Type
	cfg         interface{}
	timeout     time.Duration
	maxAttempts int
	name        string
}

func (q *delayedStarter) setInternal(atShutdown func(context.Context, func()), handle HandlerFunc, exemplar interface{}) {
	var ctx context.Context
	var cancel context.CancelFunc
	if q.timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), q.timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}

	defer cancel()
	// Ensure we also stop at shutdown
	atShutdown(ctx, func() {
		cancel()
	})

	i := 1
	for q.internal == nil {
		select {
		case <-ctx.Done():
			q.lock.Unlock()
			log.Fatal("Timedout creating queue %v with cfg %v in %s", q.underlying, q.cfg, q.name)
		default:
			queue, err := CreateQueue(q.underlying, handle, q.cfg, exemplar)
			if err == nil {
				q.internal = queue
				q.lock.Unlock()
				break
			}
			if err.Error() != "resource temporarily unavailable" {
				log.Warn("[Attempt: %d] Failed to create queue: %v for %s cfg: %v error: %v", i, q.underlying, q.name, q.cfg, err)
			}
			i++
			if q.maxAttempts > 0 && i > q.maxAttempts {
				q.lock.Unlock()
				log.Fatal("Unable to create queue %v for %s with cfg %v by max attempts: error: %v", q.underlying, q.name, q.cfg, err)
			}
			sleepTime := 100 * time.Millisecond
			if q.timeout > 0 && q.maxAttempts > 0 {
				sleepTime = (q.timeout - 200*time.Millisecond) / time.Duration(q.maxAttempts)
			}
			time.Sleep(sleepTime)
		}
	}
}

// WrappedQueue wraps a delayed starting queue
type WrappedQueue struct {
	delayedStarter
	handle   HandlerFunc
	exemplar interface{}
	channel  chan Data
}

// NewWrappedQueue will attempt to create a queue of the provided type,
// but if there is a problem creating this queue it will instead create
// a WrappedQueue with delayed the startup of the queue instead and a
// channel which will be redirected to the queue
func NewWrappedQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(WrappedQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(WrappedQueueConfiguration)

	queue, err := CreateQueue(config.Underlying, handle, config.Config, exemplar)
	if err == nil {
		// Just return the queue there is no need to wrap
		return queue, nil
	}
	if IsErrInvalidConfiguration(err) {
		// Retrying ain't gonna make this any better...
		return nil, ErrInvalidConfiguration{cfg: cfg}
	}

	return &WrappedQueue{
		handle:   handle,
		channel:  make(chan Data, config.QueueLength),
		exemplar: exemplar,
		delayedStarter: delayedStarter{
			cfg:         config.Config,
			underlying:  config.Underlying,
			timeout:     config.Timeout,
			maxAttempts: config.MaxAttempts,
			name:        config.Name,
		},
	}, nil
}

// Push will push the data to the internal channel checking it against the exemplar
func (q *WrappedQueue) Push(data Data) error {
	if q.exemplar != nil {
		// Assert data is of same type as r.exemplar
		value := reflect.ValueOf(data)
		t := value.Type()
		exemplarType := reflect.ValueOf(q.exemplar).Type()
		if !t.AssignableTo(exemplarType) || data == nil {
			return fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in %s", data, q.exemplar, q.name)
		}
	}
	q.channel <- data
	return nil
}

// Run starts to run the queue and attempts to create the internal queue
func (q *WrappedQueue) Run(atShutdown, atTerminate func(context.Context, func())) {
	q.lock.Lock()
	if q.internal == nil {
		q.setInternal(atShutdown, q.handle, q.exemplar)
		go func() {
			for data := range q.channel {
				_ = q.internal.Push(data)
			}
		}()
	} else {
		q.lock.Unlock()
	}

	q.internal.Run(atShutdown, atTerminate)
}

// Shutdown this queue and stop processing
func (q *WrappedQueue) Shutdown() {
	log.Trace("Shutdown: %s", q.name)
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
func (q *WrappedQueue) Terminate() {
	log.Trace("Terminating: %s", q.name)
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
	queuesMap[WrappedQueueType] = NewWrappedQueue
}
