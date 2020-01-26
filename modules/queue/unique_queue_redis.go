// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"

	"github.com/go-redis/redis"
)

// RedisUniqueQueueType is the type for redis queue
const RedisUniqueQueueType Type = "unique-redis"

// RedisUniqueQueue redis queue
type RedisUniqueQueue struct {
	*WorkerPool
	client     redisClient
	queueName  string
	setName    string
	closed     chan struct{}
	terminated chan struct{}
	exemplar   interface{}
	workers    int
	name       string
	lock       sync.Mutex
}

// RedisUniqueQueueConfiguration is the configuration for the redis queue
type RedisUniqueQueueConfiguration struct {
	Network      string
	Addresses    string
	Password     string
	DBIndex      int
	BatchLength  int
	QueueLength  int
	QueueName    string
	SetName      string
	Workers      int
	MaxWorkers   int
	BlockTimeout time.Duration
	BoostTimeout time.Duration
	BoostWorkers int
	Name         string
}

// NewRedisUniqueQueue creates single redis or cluster redis queue
func NewRedisUniqueQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(RedisUniqueQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(RedisUniqueQueueConfiguration)

	dbs := strings.Split(config.Addresses, ",")

	dataChan := make(chan Data, config.QueueLength)
	ctx, cancel := context.WithCancel(context.Background())

	var queue = &RedisUniqueQueue{
		WorkerPool: &WorkerPool{
			baseCtx:            ctx,
			cancel:             cancel,
			batchLength:        config.BatchLength,
			handle:             handle,
			dataChan:           dataChan,
			blockTimeout:       config.BlockTimeout,
			boostTimeout:       config.BoostTimeout,
			boostWorkers:       config.BoostWorkers,
			maxNumberOfWorkers: config.MaxWorkers,
		},
		queueName:  config.QueueName,
		setName:    config.SetName,
		exemplar:   exemplar,
		closed:     make(chan struct{}),
		terminated: make(chan struct{}),
		workers:    config.Workers,
		name:       config.Name,
	}
	if len(queue.setName) == 0 {
		queue.setName = queue.queueName + "_unique"
	}
	if len(dbs) == 0 {
		return nil, errors.New("no redis host specified")
	} else if len(dbs) == 1 {
		queue.client = redis.NewClient(&redis.Options{
			Network:  config.Network,
			Addr:     strings.TrimSpace(dbs[0]), // use default Addr
			Password: config.Password,           // no password set
			DB:       config.DBIndex,            // use default DB
		})
	} else {
		queue.client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs: dbs,
		})
	}
	if err := queue.client.Ping().Err(); err != nil {
		return nil, err
	}
	queue.qid = GetManager().Add(queue, RedisUniqueQueueType, config, exemplar)

	return queue, nil
}

// Run runs the redis queue
func (r *RedisUniqueQueue) Run(atShutdown, atTerminate func(context.Context, func())) {
	atShutdown(context.Background(), r.Shutdown)
	atTerminate(context.Background(), r.Terminate)
	log.Debug("RedisUniqueQueue: %s Starting", r.name)

	go func() {
		_ = r.AddWorkers(r.workers, 0)
	}()

	go r.readToChan()

	log.Trace("RedisUniqueQueue: %s Waiting til closed", r.name)
	<-r.closed
	log.Trace("RedisUniqueQueue: %s Waiting til done", r.name)
	r.Wait()

	log.Trace("RedisUniqueQueue: %s Waiting til cleaned", r.name)
	ctx, cancel := context.WithCancel(context.Background())
	atTerminate(ctx, cancel)
	r.CleanUp(ctx)
	cancel()
	log.Trace("RedisUniqueQueue: %s done main loop", r.name)
}

func (r *RedisUniqueQueue) readToChan() {
	for {
		select {
		case <-r.closed:
			// tell the pool to shutdown
			r.cancel()
			return
		default:
			bs, err := r.client.LPop(r.queueName).Bytes()
			if err != nil && err != redis.Nil {
				log.Error("RedisUniqueQueue: %s Error on LPop: %v", r.name, err)
				time.Sleep(time.Millisecond * 100)
				continue
			}

			if len(bs) == 0 {
				time.Sleep(time.Millisecond * 100)
				continue
			}

			data, err := unmarshalAs(bs, r.exemplar)
			if err != nil {
				log.Error("RedisUniqueQueue: %s Error on Unmarshal: %v", r.name, err)
				time.Sleep(time.Millisecond * 100)
				continue
			}
			if err := r.client.SRem(r.setName, bs).Err(); err != nil {
				log.Error("Error removing %s from uniqued set: Error: %v ", string(bs), err)
				// Will continue to process however.
			}
			log.Trace("RedisUniqueQueue: %s Task found: %#v", r.name, data)
			r.WorkerPool.Push(data)
		}
	}
}

// Push implements Queue
func (r *RedisUniqueQueue) Push(data Data) error {
	return r.PushFunc(data, nil)
}

// PushFunc implements UniqueQueue
func (r *RedisUniqueQueue) PushFunc(data Data, fn func() error) error {
	if !assignableTo(data, r.exemplar) {
		return fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in %s", data, r.exemplar, r.name)
	}
	bs, err := json.Marshal(data)
	if err != nil {
		return err
	}
	added, err := r.client.SAdd(r.setName, bs).Result()
	if err != nil {
		return err
	}
	if added == 0 {
		return ErrAlreadyInQueue
	}
	if err := fn(); err != nil {
		return err
	}

	return r.client.RPush(r.queueName, bs).Err()
}

// Has checks if the provided data is in the queue
func (r *RedisUniqueQueue) Has(data Data) (bool, error) {
	if !assignableTo(data, r.exemplar) {
		return false, fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in %s", data, r.exemplar, r.name)
	}
	bs, err := json.Marshal(data)
	if err != nil {
		return false, err
	}

	return r.client.SIsMember(r.setName, bs).Result()
}

// Shutdown processing from this queue
func (r *RedisUniqueQueue) Shutdown() {
	log.Trace("RedisUniqueQueue: %s Shutting down", r.name)
	r.lock.Lock()
	select {
	case <-r.closed:
	default:
		close(r.closed)
	}
	r.lock.Unlock()

	log.Debug("RedisUniqueQueue: %s Shutdown", r.name)
}

// Terminate this queue and close the queue
func (r *RedisUniqueQueue) Terminate() {
	log.Trace("RedisUniqueQueue: %s Terminating", r.name)
	r.Shutdown()
	r.lock.Lock()
	select {
	case <-r.terminated:
		r.lock.Unlock()
	default:
		close(r.terminated)
		r.lock.Unlock()
		if err := r.client.Close(); err != nil {
			log.Error("Error whilst closing internal redis client in %s: %v", r.name, err)
		}
	}
	log.Debug("RedisUniqueQueue: %s Terminated", r.name)
}

// Name returns the name of this queue
func (r *RedisUniqueQueue) Name() string {
	return r.name
}

func init() {
	queuesMap[RedisUniqueQueueType] = NewRedisUniqueQueue
}
