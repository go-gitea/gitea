// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// WorkerPoolQueue is a queue that uses a pool of workers to process items
// It can use different underlying (base) queue types
type WorkerPoolQueue[T any] struct {
	ctxRun       context.Context
	ctxRunCancel context.CancelFunc

	origHandler HandlerFuncT[T]
	safeHandler HandlerFuncT[T]

	baseQueueType string
	baseConfig    *BaseConfig
	baseQueue     baseQueue

	batchChan chan []T
	flushChan chan flushChanType

	batchLength     int
	workerNum       int
	workerMaxNum    int
	workerActiveNum int
	workerNumMu     sync.Mutex
}

type flushChanType chan struct{}

var _ ManagedWorkerPoolQueue = (*WorkerPoolQueue[any])(nil)

func (q *WorkerPoolQueue[T]) GetName() string {
	return q.baseConfig.ManagedName
}

func (q *WorkerPoolQueue[T]) GetType() string {
	return q.baseQueueType
}

func (q *WorkerPoolQueue[T]) GetItemTypeName() string {
	var t T
	return fmt.Sprintf("%T", t)
}

func (q *WorkerPoolQueue[T]) GetWorkerNumber() int {
	q.workerNumMu.Lock()
	defer q.workerNumMu.Unlock()
	return q.workerNum
}

func (q *WorkerPoolQueue[T]) GetWorkerActiveNumber() int {
	q.workerNumMu.Lock()
	defer q.workerNumMu.Unlock()
	return q.workerActiveNum
}

func (q *WorkerPoolQueue[T]) GetWorkerMaxNumber() int {
	q.workerNumMu.Lock()
	defer q.workerNumMu.Unlock()
	return q.workerMaxNum
}

func (q *WorkerPoolQueue[T]) SetWorkerMaxNumber(num int) {
	q.workerNumMu.Lock()
	defer q.workerNumMu.Unlock()
	q.workerMaxNum = num
}

func (q *WorkerPoolQueue[T]) GetQueueItemNumber() int {
	cnt, err := q.baseQueue.Len(q.ctxRun)
	if err != nil {
		log.Error("Failed to get number of items in queue %q: %v", q.GetName(), err)
	}
	return cnt
}

func (q *WorkerPoolQueue[T]) FlushWithContext(ctx context.Context, timeout time.Duration) (err error) {
	if q.isBaseQueueDummy() {
		return
	}

	log.Debug("Try to flush queue %q with timeout %v", q.GetName(), timeout)
	defer log.Debug("Finish flushing queue %q, err: %v", q.GetName(), err)

	var after <-chan time.Time
	after = infiniteTimerC
	if timeout > 0 {
		after = time.After(timeout)
	}
	c := make(flushChanType)

	// send flush request
	select {
	case q.flushChan <- c:
	case <-ctx.Done():
		return ctx.Err()
	case <-q.ctxRun.Done():
		return q.ctxRun.Err()
	case <-after:
		return context.DeadlineExceeded
	}

	// wait for flush to finish
	select {
	case <-c:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-q.ctxRun.Done():
		return q.ctxRun.Err()
	case <-after:
		return context.DeadlineExceeded
	}
}

func (q *WorkerPoolQueue[T]) marshal(data T) []byte {
	bs, err := json.Marshal(data)
	if err != nil {
		log.Error("Failed to marshal item for queue %q: %v", q.GetName(), err)
		return nil
	}
	return bs
}

func (q *WorkerPoolQueue[T]) unmarshal(data []byte) (t T, ok bool) {
	if err := json.Unmarshal(data, &t); err != nil {
		log.Error("Failed to unmarshal item from queue %q: %v", q.GetName(), err)
		return t, false
	}
	return t, true
}

func (q *WorkerPoolQueue[T]) isBaseQueueDummy() bool {
	_, isDummy := q.baseQueue.(*baseDummy)
	return isDummy
}

// Push adds an item to the queue, it may block for a while and then returns an error if the queue is full
func (q *WorkerPoolQueue[T]) Push(data T) error {
	if q.isBaseQueueDummy() && q.safeHandler != nil {
		// FIXME: the "immediate" queue is only for testing, but it really causes problems because its behavior is different from a real queue.
		// Even if tests pass, it doesn't mean that there is no bug in code.
		q.safeHandler(data)
	}
	return q.baseQueue.PushItem(q.ctxRun, q.marshal(data))
}

// Has only works for unique queues. Keep in mind that this check may not be reliable (due to lacking of proper transaction support)
// There could be a small chance that duplicate items appear in the queue
func (q *WorkerPoolQueue[T]) Has(data T) (bool, error) {
	return q.baseQueue.HasItem(q.ctxRun, q.marshal(data))
}

func (q *WorkerPoolQueue[T]) Run(atShutdown, atTerminate func(func())) {
	atShutdown(q.ctxRunCancel)
	q.doRun()
}

func getNewQueueFn(t string) (string, func(cfg *BaseConfig, unique bool) (baseQueue, error)) {
	switch t {
	case "dummy", "immediate":
		return t, newBaseDummy
	case "channel":
		return t, newBaseChannelGeneric
	case "redis":
		return t, newBaseRedisGeneric
	default: // level(leveldb,levelqueue,persistable-channel)
		return "levelqueue", newBaseLevelQueueGeneric
	}
}

func NewWorkerPoolQueueByIniConfig[T any](name string, queueSetting setting.QueueSettings, handler HandlerFuncT[T], unique bool) *WorkerPoolQueue[T] {
	if handler == nil {
		log.Debug("Use dummy queue for %q because handler is nil and caller doesn't want to process the queue items", name)
		queueSetting.Type = "dummy"
	}

	var w WorkerPoolQueue[T]
	var err error
	queueType, newQueueFn := getNewQueueFn(queueSetting.Type)
	w.baseQueueType = queueType
	w.baseConfig = toBaseConfig(name, queueSetting)
	w.baseQueue, err = newQueueFn(w.baseConfig, unique)
	if err != nil {
		log.Error("Failed to create queue %q: %v", name, err)
		return nil
	}
	log.Trace("Created queue %q of type %q", name, queueType)

	w.ctxRun, w.ctxRunCancel = context.WithCancel(graceful.GetManager().ShutdownContext())
	w.batchChan = make(chan []T)
	w.flushChan = make(chan flushChanType)
	w.workerMaxNum = queueSetting.MaxWorkers
	w.batchLength = queueSetting.BatchLength

	w.origHandler = handler
	w.safeHandler = func(t ...T) (unhandled []T) {
		defer func() {
			err := recover()
			if err != nil {
				log.Error("Recovered from panic in queue %q handler: %v\n%s", name, err, log.Stack(2))
			}
		}()
		return w.origHandler(t...)
	}

	return &w
}

func CreateSimpleQueue[T any](name string, handler HandlerFuncT[T]) *WorkerPoolQueue[T] {
	return createWorkerPoolQueue(name, setting.CfgProvider, handler, false)
}

func CreateUniqueQueue[T any](name string, handler HandlerFuncT[T]) *WorkerPoolQueue[T] {
	return createWorkerPoolQueue(name, setting.CfgProvider, handler, true)
}

func createWorkerPoolQueue[T any](name string, cfgProvider setting.ConfigProvider, handler HandlerFuncT[T], unique bool) *WorkerPoolQueue[T] {
	queueSetting, err := setting.GetQueueSettings(cfgProvider, name)
	if err != nil {
		log.Error("Failed to get queue settings for %q: %v", name, err)
		return nil
	}
	w := NewWorkerPoolQueueByIniConfig(name, queueSetting, handler, unique)
	GetManager().AddManagedQueue(w)
	return w
}
