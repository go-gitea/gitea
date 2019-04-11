// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

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
	client      redisClient
	queueName   string
	indexer     Indexer
	batchNumber int
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
func NewRedisQueue(addrs string, password string, dbIdx int, indexer Indexer, batchNumber int) (*RedisQueue, error) {
	dbs := strings.Split(addrs, ",")
	var queue = RedisQueue{
		queueName:   "issue_indexer_queue",
		indexer:     indexer,
		batchNumber: batchNumber,
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
		queue.client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs: dbs,
		})
	}
	if err := queue.client.Ping().Err(); err != nil {
		return nil, err
	}
	return &queue, nil
}

// Run runs the redis queue
func (r *RedisQueue) Run() error {
	var i int
	var datas = make([]*IndexerData, 0, r.batchNumber)
	for {
		bs, err := r.client.LPop(r.queueName).Bytes()
		if err != nil && err != redis.Nil {
			log.Error("LPop faile: %v", err)
			time.Sleep(time.Millisecond * 100)
			continue
		}

		i++
		if len(datas) > r.batchNumber || (len(datas) > 0 && i > 3) {
			_ = r.indexer.Index(datas)
			datas = make([]*IndexerData, 0, r.batchNumber)
			i = 0
		}

		if len(bs) == 0 {
			time.Sleep(time.Millisecond * 100)
			continue
		}

		var data IndexerData
		err = json.Unmarshal(bs, &data)
		if err != nil {
			log.Error("Unmarshal: %v", err)
			time.Sleep(time.Millisecond * 100)
			continue
		}

		log.Trace("RedisQueue: task found: %#v", data)

		if data.IsDelete {
			if data.ID > 0 {
				if err = r.indexer.Delete(data.ID); err != nil {
					log.Error("indexer.Delete: %v", err)
				}
			} else if len(data.IDs) > 0 {
				if err = r.indexer.Delete(data.IDs...); err != nil {
					log.Error("indexer.Delete: %v", err)
				}
			}
			time.Sleep(time.Millisecond * 100)
			continue
		}

		datas = append(datas, &data)
		time.Sleep(time.Millisecond * 100)
	}
}

// Push implements Queue
func (r *RedisQueue) Push(data *IndexerData) error {
	bs, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return r.client.RPush(r.queueName, bs).Err()
}
