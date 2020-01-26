// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"

	"github.com/go-redis/redis"
)

// RedisQueueType is the type for redis queue
const RedisQueueType Type = "redis"

type redisClient interface {
	RPush(key string, args ...interface{}) *redis.IntCmd
	LPop(key string) *redis.StringCmd
	Ping() *redis.StatusCmd
	Close() error
}

// RedisQueue redis queue
type RedisQueue struct {
	pool       *WorkerPool
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
	Network      string
	Addresses    string
	Password     string
	DBIndex      int
	BatchLength  int
	QueueLength  int
	QueueName    string
	Workers      int
	MaxWorkers   int
	BlockTimeout time.Duration
	BoostTimeout time.Duration
	BoostWorkers int
	Name         string
}

// NewRedisQueue creates single redis or cluster redis queue
func NewRedisQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(RedisQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(RedisQueueConfiguration)

	dbs := strings.Split(config.Addresses, ",")

	dataChan := make(chan Data, config.QueueLength)
	ctx, cancel := context.WithCancel(context.Background())

	var queue = &RedisQueue{
		pool: &WorkerPool{
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
	queue.pool.qid = GetManager().Add(queue, RedisQueueType, config, exemplar, queue.pool)

	return queue, nil
}

// Run runs the redis queue
func (r *RedisQueue) Run(atShutdown, atTerminate func(context.Context, func())) {
	atShutdown(context.Background(), r.Shutdown)
	atTerminate(context.Background(), r.Terminate)

	go func() {
		_ = r.pool.AddWorkers(r.workers, 0)
	}()

	go r.readToChan()

	log.Trace("RedisQueue: %s Waiting til closed", r.name)
	<-r.closed
	log.Trace("RedisQueue: %s Waiting til done", r.name)
	r.pool.Wait()

	log.Trace("RedisQueue: %s Waiting til cleaned", r.name)
	ctx, cancel := context.WithCancel(context.Background())
	atTerminate(ctx, cancel)
	r.pool.CleanUp(ctx)
	cancel()
}

func (r *RedisQueue) readToChan() {
	for {
		select {
		case <-r.closed:
			// tell the pool to shutdown
			r.pool.cancel()
			return
		default:
			bs, err := r.client.LPop(r.queueName).Bytes()
			if err != nil && err != redis.Nil {
				log.Error("RedisQueue: %s Error on LPop: %v", r.name, err)
				time.Sleep(time.Millisecond * 100)
				continue
			}

			if len(bs) == 0 {
				time.Sleep(time.Millisecond * 100)
				continue
			}

			var data Data
			if r.exemplar != nil {
				t := reflect.TypeOf(r.exemplar)
				n := reflect.New(t)
				ne := n.Elem()
				err = json.Unmarshal(bs, ne.Addr().Interface())
				data = ne.Interface().(Data)
			} else {
				err = json.Unmarshal(bs, &data)
			}
			if err != nil {
				log.Error("RedisQueue: %s Error on Unmarshal: %v", r.name, err)
				time.Sleep(time.Millisecond * 100)
				continue
			}

			log.Trace("RedisQueue: %s Task found: %#v", r.name, data)
			r.pool.Push(data)
		}
	}
}

// Push implements Queue
func (r *RedisQueue) Push(data Data) error {
	if r.exemplar != nil {
		// Assert data is of same type as r.exemplar
		value := reflect.ValueOf(data)
		t := value.Type()
		exemplarType := reflect.ValueOf(r.exemplar).Type()
		if !t.AssignableTo(exemplarType) || data == nil {
			return fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in %s", data, r.exemplar, r.name)
		}
	}
	bs, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return r.client.RPush(r.queueName, bs).Err()
}

// Shutdown processing from this queue
func (r *RedisQueue) Shutdown() {
	log.Trace("Shutdown: %s", r.name)
	r.lock.Lock()
	select {
	case <-r.closed:
	default:
		close(r.closed)
	}
	r.lock.Unlock()
}

// Terminate this queue and close the queue
func (r *RedisQueue) Terminate() {
	log.Trace("Terminating: %s", r.name)
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
}

// Name returns the name of this queue
func (r *RedisQueue) Name() string {
	return r.name
}

func init() {
	queuesMap[RedisQueueType] = NewRedisQueue
}
