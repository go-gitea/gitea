// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/bradfitz/gomemcache/memcache"
)

const memcacheMaxRelativeTTL = 30 * 24 * 60 * 60

type memcacheCache struct {
	client *memcache.Client
}

func newMemcacheCache(conn string) (backend, error) {
	if conn == "" {
		return nil, errors.New("cache adapter memcache requires [cache] HOST, e.g. 127.0.0.1:11211")
	}
	servers := &memcache.ServerList{}
	if err := servers.SetServers(strings.Split(conn, ";")...); err != nil {
		return nil, fmt.Errorf("invalid memcache HOST %q: %w", conn, err)
	}
	return &memcacheCache{client: memcache.NewFromSelector(servers)}, nil
}

// memcacheExpiration converts a TTL beyond memcached's 30 day relative limit into an absolute unix time
func memcacheExpiration(ttl int64) int32 {
	if ttl > memcacheMaxRelativeTTL {
		ttl += timeNow().Unix()
	}
	return int32(min(ttl, math.MaxInt32))
}

func (c *memcacheCache) Get(key string) (string, bool) {
	item, err := c.client.Get(key)
	if err != nil {
		return "", false
	}
	return string(item.Value), true
}

func (c *memcacheCache) GetAndDelete(key string) (string, bool) {
	item, err := c.client.Get(key)
	if err != nil || c.client.Delete(key) != nil {
		return "", false
	}
	return string(item.Value), true
}

func (c *memcacheCache) Put(key, value string, ttl int64) error {
	return c.client.Set(&memcache.Item{Key: key, Value: []byte(value), Expiration: memcacheExpiration(ttl)})
}

func (c *memcacheCache) Delete(key string) error {
	if err := c.client.Delete(key); err != nil && !errors.Is(err, memcache.ErrCacheMiss) {
		return err
	}
	return nil
}

func (c *memcacheCache) IsExist(key string) bool {
	_, err := c.client.Get(key)
	return err == nil
}

func (c *memcacheCache) Ping() error {
	const key = "__gitea_cache_ping"
	if err := c.Put(key, "ping", 10); err != nil {
		return err
	}
	return c.Delete(key)
}
