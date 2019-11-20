// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"

	"gitea.com/lunny/levelqueue"
)

// LevelQueueType is the type for level queue
const LevelQueueType Type = "level"

// LevelQueueConfiguration is the configuration for a LevelQueue
type LevelQueueConfiguration struct {
	DataDir     string
	BatchLength int
	Workers     int
}

// LevelQueue implements a disk library queue
type LevelQueue struct {
	handle      HandlerFunc
	queue       *levelqueue.Queue
	batchLength int
	closed      chan struct{}
	exemplar    interface{}
	workers     int
}

// NewLevelQueue creates a ledis local queue
func NewLevelQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(LevelQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(LevelQueueConfiguration)

	queue, err := levelqueue.Open(config.DataDir)
	if err != nil {
		return nil, err
	}

	return &LevelQueue{
		handle:      handle,
		queue:       queue,
		batchLength: config.BatchLength,
		exemplar:    exemplar,
		closed:      make(chan struct{}),
		workers:     config.Workers,
	}, nil
}

// Run starts to run the queue
func (l *LevelQueue) Run(atShutdown, atTerminate func(context.Context, func())) {
	atShutdown(context.Background(), l.Shutdown)
	atTerminate(context.Background(), l.Terminate)

	wg := sync.WaitGroup{}
	for i := 0; i < l.workers; i++ {
		wg.Add(1)
		go func() {
			l.worker()
			wg.Done()
		}()
	}
	wg.Wait()
}

func (l *LevelQueue) worker() {
	var i int
	var datas = make([]Data, 0, l.batchLength)
	for {
		select {
		case <-l.closed:
			if len(datas) > 0 {
				log.Trace("Handling: %d data, %v", len(datas), datas)
				l.handle(datas...)
			}
			return
		default:
		}
		i++
		if len(datas) > l.batchLength || (len(datas) > 0 && i > 3) {
			log.Trace("Handling: %d data, %v", len(datas), datas)
			l.handle(datas...)
			datas = make([]Data, 0, l.batchLength)
			i = 0
			continue
		}

		bs, err := l.queue.RPop()
		if err != nil {
			if err != levelqueue.ErrNotFound {
				log.Error("RPop: %v", err)
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
			log.Error("Unmarshal: %v", err)
			time.Sleep(time.Millisecond * 10)
			continue
		}

		log.Trace("LevelQueue: task found: %#v", data)

		datas = append(datas, data)
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
	select {
	case <-l.closed:
	default:
		close(l.closed)
	}
}

// Terminate this queue and close the queue
func (l *LevelQueue) Terminate() {
	l.Shutdown()
	if err := l.queue.Close(); err != nil && err.Error() != "leveldb: closed" {
		log.Error("Error whilst closing internal queue: %v", err)
	}

}

func init() {
	queuesMap[LevelQueueType] = NewLevelQueue
}
