// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cache

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/nosql"

	"gitea.com/macaron/cache"
	"github.com/go-redis/redis/v7"
	"github.com/unknwon/com"
)

// RedisCacher represents a redis cache adapter implementation.
type RedisCacher struct {
	c          redis.UniversalClient
	prefix     string
	hsetName   string
	occupyMode bool
}

// Put puts value into cache with key and expire time.
// If expired is 0, it lives forever.
func (c *RedisCacher) Put(key string, val interface{}, expire int64) error {
	key = c.prefix + key
	if expire == 0 {
		if err := c.c.Set(key, com.ToStr(val), 0).Err(); err != nil {
			return err
		}
	} else {
		dur, err := time.ParseDuration(com.ToStr(expire) + "s")
		if err != nil {
			return err
		}
		if err = c.c.Set(key, com.ToStr(val), dur).Err(); err != nil {
			return err
		}
	}

	if c.occupyMode {
		return nil
	}
	return c.c.HSet(c.hsetName, key, "0").Err()
}

// Get gets cached value by given key.
func (c *RedisCacher) Get(key string) interface{} {
	val, err := c.c.Get(c.prefix + key).Result()
	if err != nil {
		return nil
	}
	return val
}

// Delete deletes cached value by given key.
func (c *RedisCacher) Delete(key string) error {
	key = c.prefix + key
	if err := c.c.Del(key).Err(); err != nil {
		return err
	}

	if c.occupyMode {
		return nil
	}
	return c.c.HDel(c.hsetName, key).Err()
}

// Incr increases cached int-type value by given key as a counter.
func (c *RedisCacher) Incr(key string) error {
	if !c.IsExist(key) {
		return fmt.Errorf("key '%s' not exist", key)
	}
	return c.c.Incr(c.prefix + key).Err()
}

// Decr decreases cached int-type value by given key as a counter.
func (c *RedisCacher) Decr(key string) error {
	if !c.IsExist(key) {
		return fmt.Errorf("key '%s' not exist", key)
	}
	return c.c.Decr(c.prefix + key).Err()
}

// IsExist returns true if cached value exists.
func (c *RedisCacher) IsExist(key string) bool {
	if c.c.Exists(c.prefix+key).Val() == 1 {
		return true
	}

	if !c.occupyMode {
		c.c.HDel(c.hsetName, c.prefix+key)
	}
	return false
}

// Flush deletes all cached data.
func (c *RedisCacher) Flush() error {
	if c.occupyMode {
		return c.c.FlushDB().Err()
	}

	keys, err := c.c.HKeys(c.hsetName).Result()
	if err != nil {
		return err
	}
	if err = c.c.Del(keys...).Err(); err != nil {
		return err
	}
	return c.c.Del(c.hsetName).Err()
}

// StartAndGC starts GC routine based on config string settings.
// AdapterConfig: network=tcp,addr=:6379,password=macaron,db=0,pool_size=100,idle_timeout=180,hset_name=MacaronCache,prefix=cache:
func (c *RedisCacher) StartAndGC(opts cache.Options) error {
	c.hsetName = "MacaronCache"
	c.occupyMode = opts.OccupyMode

	uri := nosql.ToRedisURI(opts.AdapterConfig)

	c.c = nosql.GetManager().GetRedisClient(uri.String())

	for k, v := range uri.Query() {
		switch k {
		case "hset_name":
			c.hsetName = v[0]
		case "prefix":
			c.prefix = v[0]
		}
	}

	return c.c.Ping().Err()
}

func init() {
	cache.Register("redis", &RedisCacher{})
}
