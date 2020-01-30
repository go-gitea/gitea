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
	"sync/atomic"
	"time"

	"code.gitea.io/gitea/modules/log"

	"github.com/go-redis/redis"
)

// RedisQueueType is the type for redis queue
const RedisQueueType Type = "redis"

type redisClient interface {
	RPush(key string, args ...interface{}) *redis.IntCmd
	LPop(key string) *redis.StringCmd
	LLen(key string) *redis.IntCmd
	Ping() *redis.StatusCmd
	Close() error
}

// RedisQueue redis queue
type RedisQueue struct {
	*WorkerPool
	client     redisClient
	queueName  string
	closed     chan struct{}
	terminated chan struct{}
	exemplar   interface{}
	workers    int
	name       string
	lock       sync.Mutex
}

// RedisQueueConfiguration is the configuration for the redis queue
type RedisQueueConfiguration struct {
	WorkerPoolConfiguration
	Network   string
	Addresses string
	Password  string
	DBIndex   int
	QueueName string
	Workers   int
	Name      string
}

// NewRedisQueue creates single redis or cluster redis queue
func NewRedisQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(RedisQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(RedisQueueConfiguration)

	dbs := strings.Split(config.Addresses, ",")

	var queue = &RedisQueue{
		WorkerPool: NewWorkerPool(handle, config.WorkerPoolConfiguration),
		queueName:  config.QueueName,
		exemplar:   exemplar,
		closed:     make(chan struct{}),
		terminated: make(chan struct{}),
		workers:    config.Workers,
		name:       config.Name,
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
	queue.qid = GetManager().Add(queue, RedisQueueType, config, exemplar)

	return queue, nil
}

// Run runs the redis queue
func (r *RedisQueue) Run(atShutdown, atTerminate func(context.Context, func())) {
	atShutdown(context.Background(), r.Shutdown)
	atTerminate(context.Background(), r.Terminate)
	log.Debug("RedisQueue: %s Starting", r.name)

	go func() {
		_ = r.AddWorkers(r.workers, 0)
	}()

	go r.readToChan()

	log.Trace("RedisQueue: %s Waiting til closed", r.name)
	<-r.closed
	log.Trace("RedisQueue: %s Waiting til done", r.name)
	r.Wait()

	log.Trace("RedisQueue: %s Waiting til cleaned", r.name)
	ctx, cancel := context.WithCancel(context.Background())
	atTerminate(ctx, cancel)
	r.CleanUp(ctx)
	cancel()
}

func (r *RedisQueue) readToChan() {
	for {
		select {
		case <-r.closed:
			// tell the pool to shutdown
			r.cancel()
			return
		default:
			atomic.AddInt64(&r.numInQueue, 1)
			bs, err := r.client.LPop(r.queueName).Bytes()
			if err != nil && err != redis.Nil {
				log.Error("RedisQueue: %s Error on LPop: %v", r.name, err)
				atomic.AddInt64(&r.numInQueue, -1)
				time.Sleep(time.Millisecond * 100)
				continue
			}

			if len(bs) == 0 {
				atomic.AddInt64(&r.numInQueue, -1)
				time.Sleep(time.Millisecond * 100)
				continue
			}

			data, err := unmarshalAs(bs, r.exemplar)
			if err != nil {
				log.Error("RedisQueue: %s Error on Unmarshal: %v", r.name, err)
				atomic.AddInt64(&r.numInQueue, -1)
				time.Sleep(time.Millisecond * 100)
				continue
			}

			log.Trace("RedisQueue: %s Task found: %#v", r.name, data)
			r.WorkerPool.Push(data)
			atomic.AddInt64(&r.numInQueue, -1)
		}
	}
}

// Push implements Queue
func (r *RedisQueue) Push(data Data) error {
	if !assignableTo(data, r.exemplar) {
		return fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in %s", data, r.exemplar, r.name)
	}
	bs, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return r.client.RPush(r.queueName, bs).Err()
}

// IsEmpty checks if the queue is empty
func (r *RedisQueue) IsEmpty() bool {
	if !r.WorkerPool.IsEmpty() {
		return false
	}
	length, err := r.client.LLen(r.queueName).Result()
	if err != nil {
		log.Error("Error whilst getting queue length for %s: Error: %v", r.name, err)
		return false
	}
	return length == 0
}

// Shutdown processing from this queue
func (r *RedisQueue) Shutdown() {
	log.Trace("RedisQueue: %s Shutting down", r.name)
	r.lock.Lock()
	select {
	case <-r.closed:
	default:
		close(r.closed)
	}
	r.lock.Unlock()
	log.Debug("RedisQueue: %s Shutdown", r.name)
}

// Terminate this queue and close the queue
func (r *RedisQueue) Terminate() {
	log.Trace("RedisQueue: %s Terminating", r.name)
	r.Shutdown()
	r.lock.Lock()
	select {
	case <-r.terminated:
		r.lock.Unlock()
	default:
		close(r.terminated)
		r.lock.Unlock()
		if log.IsDebug() {
			log.Debug("RedisQueue: %s Closing with %d tasks left in queue", r.name, r.client.LLen(r.queueName))
		}
		if err := r.client.Close(); err != nil {
			log.Error("Error whilst closing internal redis client in %s: %v", r.name, err)
		}
	}
	log.Debug("RedisQueue: %s Terminated", r.name)
}

// Name returns the name of this queue
func (r *RedisQueue) Name() string {
	return r.name
}

func init() {
	queuesMap[RedisQueueType] = NewRedisQueue
}
