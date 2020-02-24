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
	"strings"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/unknwon/com"

	"gitea.com/macaron/cache"
)

// MemcacheCacher represents a memcache cache adapter implementation.
type MemcacheCacher struct {
	c *memcache.Client
}

func NewItem(key string, data []byte, expire int32) *memcache.Item {
	return &memcache.Item{
		Key:        key,
		Value:      data,
		Expiration: expire,
	}
}

// Put puts value into cache with key and expire time.
// If expired is 0, it lives forever.
func (c *MemcacheCacher) Put(key string, val interface{}, expire int64) error {
	return c.c.Set(NewItem(key, []byte(com.ToStr(val)), int32(expire)))
}

// Get gets cached value by given key.
func (c *MemcacheCacher) Get(key string) interface{} {
	item, err := c.c.Get(key)
	if err != nil {
		return nil
	}
	return string(item.Value)
}

// Delete deletes cached value by given key.
func (c *MemcacheCacher) Delete(key string) error {
	return c.c.Delete(key)
}

// Incr increases cached int-type value by given key as a counter.
func (c *MemcacheCacher) Incr(key string) error {
	_, err := c.c.Increment(key, 1)
	return err
}

// Decr decreases cached int-type value by given key as a counter.
func (c *MemcacheCacher) Decr(key string) error {
	_, err := c.c.Decrement(key, 1)
	return err
}

// IsExist returns true if cached value exists.
func (c *MemcacheCacher) IsExist(key string) bool {
	_, err := c.c.Get(key)
	return err == nil
}

// Flush deletes all cached data.
func (c *MemcacheCacher) Flush() error {
	return c.c.FlushAll()
}

// StartAndGC starts GC routine based on config string settings.
// AdapterConfig: 127.0.0.1:9090;127.0.0.1:9091
func (c *MemcacheCacher) StartAndGC(opt cache.Options) error {
	c.c = memcache.New(strings.Split(opt.AdapterConfig, ";")...)
	return nil
}

func init() {
	cache.Register("memcache", &MemcacheCacher{})
}
