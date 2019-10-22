// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package task

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"

	"github.com/go-redis/redis"
)

var (
	_ Queue = &RedisQueue{}
)

type redisClient interface {
	RPush(key string, args ...interface{}) *redis.IntCmd
	LPop(key string) *redis.StringCmd
	Ping() *redis.StatusCmd
}

// RedisQueue redis queue
type RedisQueue struct {
	client    redisClient
	queueName string
	closeChan chan bool
}

func parseConnStr(connStr string) (addrs, password string, dbIdx int, err error) {
	fields := strings.Fields(connStr)
	for _, f := range fields {
		items := strings.SplitN(f, "=", 2)
		if len(items) < 2 {
			continue
		}
		switch strings.ToLower(items[0]) {
		case "addrs":
			addrs = items[1]
		case "password":
			password = items[1]
		case "db":
			dbIdx, err = strconv.Atoi(items[1])
			if err != nil {
				return
			}
		}
	}
	return
}

// NewRedisQueue creates single redis or cluster redis queue
func NewRedisQueue(addrs string, password string, dbIdx int) (*RedisQueue, error) {
	dbs := strings.Split(addrs, ",")
	var queue = RedisQueue{
		queueName: "task_queue",
		closeChan: make(chan bool),
	}
	if len(dbs) == 0 {
		return nil, errors.New("no redis host found")
	} else if len(dbs) == 1 {
		queue.client = redis.NewClient(&redis.Options{
			Addr:     strings.TrimSpace(dbs[0]), // use default Addr
			Password: password,                  // no password set
			DB:       dbIdx,                     // use default DB
		})
	} else {
		// cluster will ignore db
		queue.client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    dbs,
			Password: password,
		})
	}
	if err := queue.client.Ping().Err(); err != nil {
		return nil, err
	}
	return &queue, nil
}

// Run starts to run the queue
func (r *RedisQueue) Run() error {
	for {
		select {
		case <-r.closeChan:
			return nil
		case <-time.After(time.Millisecond * 100):
		}

		bs, err := r.client.LPop(r.queueName).Bytes()
		if err != nil {
			if err != redis.Nil {
				log.Error("LPop failed: %v", err)
			}
			time.Sleep(time.Millisecond * 100)
			continue
		}

		var task models.Task
		err = json.Unmarshal(bs, &task)
		if err != nil {
			log.Error("Unmarshal task failed: %s", err.Error())
		} else {
			err = Run(&task)
			if err != nil {
				log.Error("Run task failed: %s", err.Error())
			}
		}
	}
}

// Push implements Queue
func (r *RedisQueue) Push(task *models.Task) error {
	bs, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return r.client.RPush(r.queueName, bs).Err()
}

// Stop stop the queue
func (r *RedisQueue) Stop() {
	r.closeChan <- true
}
