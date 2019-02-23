// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lstree

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/git"

	"github.com/go-redis/redis"
)

type redisClient interface {
	Get(key string) *redis.StringCmd
	Set(key string, value interface{}, expiration time.Duration) *redis.StatusCmd
}

// RedisCache redis
type RedisCache struct {
	client redisClient
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

// NewRedisCache creates single redis or cluster redis client
func NewRedisCache(addrs string, password string, dbIdx int) (*RedisCache, error) {
	dbs := strings.Split(addrs, ",")
	var cache RedisCache
	if len(dbs) == 0 {
		return nil, errors.New("no redis host found")
	} else if len(dbs) == 1 {
		cache.client = redis.NewClient(&redis.Options{
			Addr:     strings.TrimSpace(dbs[0]), // use default Addr
			Password: password,                  // no password set
			DB:       dbIdx,                     // use default DB
		})
	} else {
		cache.client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs: dbs,
		})
	}
	return &cache, nil
}

// Get implements git.LastCommitCache
func (r *RedisCache) Get(repoPath, treeIsh string) (git.Entries, error) {
	bs, err := r.client.Get(getKey(repoPath, treeIsh)).Bytes()
	if err != nil {
		return nil, err
	}
	var entries git.Entries
	err = json.Unmarshal(bs, &entries)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// Put implements git.LastCommitCache
func (r *RedisCache) Put(repoPath, treeIsh string, entries git.Entries) error {
	bs, err := json.Marshal(entries)
	if err != nil {
		return err
	}

	return r.client.Set(getKey(repoPath, treeIsh), bs, 10*time.Second).Err()
}
