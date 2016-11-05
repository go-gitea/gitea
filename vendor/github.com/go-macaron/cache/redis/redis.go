// Copyright 2013 Beego Authors
// Copyright 2014 The Macaron Authors
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package cache

import (
	"fmt"
	"strings"
	"time"

	"github.com/Unknwon/com"
	"gopkg.in/ini.v1"
	"gopkg.in/redis.v2"

	"github.com/go-macaron/cache"
)

// RedisCacher represents a redis cache adapter implementation.
type RedisCacher struct {
	c          *redis.Client
	prefix     string
	hsetName   string
	occupyMode bool
}

// Put puts value into cache with key and expire time.
// If expired is 0, it lives forever.
func (c *RedisCacher) Put(key string, val interface{}, expire int64) error {
	key = c.prefix + key
	if expire == 0 {
		if err := c.c.Set(key, com.ToStr(val)).Err(); err != nil {
			return err
		}
	} else {
		dur, err := time.ParseDuration(com.ToStr(expire) + "s")
		if err != nil {
			return err
		}
		if err = c.c.SetEx(key, dur, com.ToStr(val)).Err(); err != nil {
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
	if c.c.Exists(c.prefix + key).Val() {
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
		return c.c.FlushDb().Err()
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

	cfg, err := ini.Load([]byte(strings.Replace(opts.AdapterConfig, ",", "\n", -1)))
	if err != nil {
		return err
	}

	opt := &redis.Options{
		Network: "tcp",
	}
	for k, v := range cfg.Section("").KeysHash() {
		switch k {
		case "network":
			opt.Network = v
		case "addr":
			opt.Addr = v
		case "password":
			opt.Password = v
		case "db":
			opt.DB = com.StrTo(v).MustInt64()
		case "pool_size":
			opt.PoolSize = com.StrTo(v).MustInt()
		case "idle_timeout":
			opt.IdleTimeout, err = time.ParseDuration(v + "s")
			if err != nil {
				return fmt.Errorf("error parsing idle timeout: %v", err)
			}
		case "hset_name":
			c.hsetName = v
		case "prefix":
			c.prefix = v
		default:
			return fmt.Errorf("session/redis: unsupported option '%s'", k)
		}
	}

	c.c = redis.NewClient(opt)
	if err = c.c.Ping().Err(); err != nil {
		return err
	}

	return nil
}

func init() {
	cache.Register("redis", &RedisCacher{})
}
