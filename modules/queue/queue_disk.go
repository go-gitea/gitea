// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"code.gitea.io/gitea/modules/log"

	"gitea.com/lunny/levelqueue"
)

// LevelQueueType is the type for level queue
const LevelQueueType Type = "level"

// LevelQueueConfiguration is the configuration for a LevelQueue
type LevelQueueConfiguration struct {
	DataDir      string
	QueueLength  int
	BatchLength  int
	Workers      int
	BlockTimeout time.Duration
	BoostTimeout time.Duration
	BoostWorkers int
	Name         string
}

// LevelQueue implements a disk library queue
type LevelQueue struct {
	pool       *WorkerPool
	queue      *levelqueue.Queue
	closed     chan struct{}
	terminated chan struct{}
	exemplar   interface{}
	workers    int
	name       string
}

// NewLevelQueue creates a ledis local queue
func NewLevelQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(LevelQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(LevelQueueConfiguration)

	internal, err := levelqueue.Open(config.DataDir)
	if err != nil {
		return nil, err
	}

	dataChan := make(chan Data, config.QueueLength)
	ctx, cancel := context.WithCancel(context.Background())

	queue := &LevelQueue{
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
		queue:      internal,
		exemplar:   exemplar,
		closed:     make(chan struct{}),
		terminated: make(chan struct{}),
		workers:    config.Workers,
		name:       config.Name,
	}
	queue.pool.qid = GetManager().Add(queue, LevelQueueType, config, exemplar, queue.pool.AddWorkers, queue.pool.NumberOfWorkers)
	return queue, nil
}

// Run starts to run the queue
func (l *LevelQueue) Run(atShutdown, atTerminate func(context.Context, func())) {
	atShutdown(context.Background(), l.Shutdown)
	atTerminate(context.Background(), l.Terminate)

	go func() {
		_ = l.pool.AddWorkers(l.workers, 0)
	}()

	go l.readToChan()

	log.Trace("%s Waiting til closed", l.name)
	<-l.closed

	log.Trace("%s Waiting til done", l.name)
	l.pool.Wait()
	// FIXME: graceful: Needs HammerContext
	log.Trace("%s Waiting til cleaned", l.name)

	l.pool.CleanUp(context.TODO())
	log.Trace("%s cleaned", l.name)

}

func (l *LevelQueue) readToChan() {
	for {
		select {
		case <-l.closed:
			// tell the pool to shutdown.
			l.pool.cancel()
			return
		default:
			bs, err := l.queue.RPop()
			if err != nil {
				if err != levelqueue.ErrNotFound {
					log.Error("%s RPop: %v", l.name, err)
				}
				time.Sleep(time.Millisecond * 100)
				continue
			}

			if len(bs) == 0 {
				time.Sleep(time.Millisecond * 100)
				continue
			}

			var data Data
			if l.exemplar != nil {
				t := reflect.TypeOf(l.exemplar)
				n := reflect.New(t)
				ne := n.Elem()
				err = json.Unmarshal(bs, ne.Addr().Interface())
				data = ne.Interface().(Data)
			} else {
				err = json.Unmarshal(bs, &data)
			}
			if err != nil {
				log.Error("LevelQueue: %s failed to unmarshal: %v", l.name, err)
				time.Sleep(time.Millisecond * 10)
				continue
			}

			log.Trace("LevelQueue %s: task found: %#v", l.name, data)
			l.pool.Push(data)
			time.Sleep(time.Millisecond * 100)

		}
	}
}

// Push will push the indexer data to queue
func (l *LevelQueue) Push(data Data) error {
	if l.exemplar != nil {
		// Assert data is of same type as r.exemplar
		value := reflect.ValueOf(data)
		t := value.Type()
		exemplarType := reflect.ValueOf(l.exemplar).Type()
		if !t.AssignableTo(exemplarType) || data == nil {
			return fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in %s", data, l.exemplar, l.name)
		}
	}
	bs, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return l.queue.LPush(bs)
}

// Shutdown this queue and stop processing
func (l *LevelQueue) Shutdown() {
	log.Trace("Shutdown: %s", l.name)
	select {
	case <-l.closed:
	default:
		close(l.closed)
	}
}

// Terminate this queue and close the queue
func (l *LevelQueue) Terminate() {
	log.Trace("Terminating: %s", l.name)
	l.Shutdown()
	if err := l.queue.Close(); err != nil && err.Error() != "leveldb: closed" {
		log.Error("Error whilst closing internal queue in %s: %v", l.name, err)
	}
}

// Name returns the name of this queue
func (l *LevelQueue) Name() string {
	return l.name
}

func init() {
	queuesMap[LevelQueueType] = NewLevelQueue
}
