// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"time"

	"gitea.dev/modules/graceful"
	"gitea.dev/modules/nosql"

	"github.com/redis/go-redis/v9"
)

type redisCache struct {
	client redis.UniversalClient
	prefix string
}

func newRedisCache(conn string) backend {
	uri := nosql.ToRedisURI(conn)
	return &redisCache{
		client: nosql.GetManager().GetRedisClient(uri.String()),
		prefix: uri.Query().Get("prefix"),
	}
}

func (c *redisCache) Get(key string) (string, bool) {
	value, err := c.client.Get(graceful.GetManager().HammerContext(), c.prefix+key).Result()
	return value, err == nil
}

func (c *redisCache) GetAndDelete(key string) (string, bool) {
	ctx := graceful.GetManager().HammerContext()
	var get *redis.StringCmd
	_, err := c.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		get = pipe.Get(ctx, c.prefix+key)
		pipe.Del(ctx, c.prefix+key)
		return nil
	})
	return get.Val(), err == nil
}

func (c *redisCache) Put(key, value string, ttl int64) error {
	return c.client.Set(graceful.GetManager().HammerContext(), c.prefix+key, value, time.Duration(ttl)*time.Second).Err()
}

func (c *redisCache) Delete(key string) error {
	return c.client.Del(graceful.GetManager().HammerContext(), c.prefix+key).Err()
}

func (c *redisCache) IsExist(key string) bool {
	return c.client.Exists(graceful.GetManager().HammerContext(), c.prefix+key).Val() == 1
}

func (c *redisCache) Ping() error {
	return c.client.Ping(graceful.GetManager().HammerContext()).Err()
}
