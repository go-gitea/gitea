// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"gitea.dev/modules/json"

	lru "github.com/hashicorp/golang-lru/v2"
)

const twoQueueDefaultSize = 50000

type twoQueueCache struct {
	mutex sync.Mutex // makes check-then-remove atomic against concurrent puts
	cache *lru.TwoQueueCache[string, memoryItem]
}

type twoQueueCacheConfig struct {
	Size        int     `json:"size"`
	RecentRatio float64 `json:"recent_ratio"`
	GhostRatio  float64 `json:"ghost_ratio"`
}

func newTwoQueueCache(conn string, gcInterval time.Duration) (backend, error) {
	lruCache, err := newTwoQueueLRU(conn)
	if err != nil {
		return nil, err
	}
	cache := &twoQueueCache{cache: lruCache}
	startGC(gcInterval, cache.deleteExpired)
	return cache, nil
}

func newTwoQueueLRU(conn string) (*lru.TwoQueueCache[string, memoryItem], error) {
	if conn == "" {
		return lru.New2Q[string, memoryItem](twoQueueDefaultSize)
	}
	if size, err := strconv.Atoi(conn); err == nil {
		return lru.New2Q[string, memoryItem](size)
	}
	if !json.Valid([]byte(conn)) {
		return nil, fmt.Errorf("invalid twoqueue cache HOST %q, expected a size or a JSON config", conn)
	}
	config := twoQueueCacheConfig{
		Size:        twoQueueDefaultSize,
		RecentRatio: lru.Default2QRecentRatio,
		GhostRatio:  lru.Default2QGhostEntries,
	}
	_ = json.Unmarshal([]byte(conn), &config)
	return lru.New2QParams[string, memoryItem](config.Size, config.RecentRatio, config.GhostRatio)
}

func (c *twoQueueCache) Get(key string) (string, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	item, ok := c.cache.Get(key)
	if !ok {
		return "", false
	}
	if item.expired(timeNow()) {
		c.cache.Remove(key)
		return "", false
	}
	return item.value, true
}

func (c *twoQueueCache) GetAndDelete(key string) (string, bool) {
	c.mutex.Lock()
	item, ok := c.cache.Peek(key)
	c.cache.Remove(key)
	c.mutex.Unlock()
	return item.value, ok && !item.expired(timeNow())
}

func (c *twoQueueCache) Put(key, value string, ttl int64) error {
	item := newMemoryItem(value, ttl)
	c.mutex.Lock()
	c.cache.Add(key, item)
	c.mutex.Unlock()
	return nil
}

func (c *twoQueueCache) Delete(key string) error {
	c.mutex.Lock()
	c.cache.Remove(key)
	c.mutex.Unlock()
	return nil
}

func (c *twoQueueCache) IsExist(key string) bool {
	c.mutex.Lock()
	item, ok := c.cache.Peek(key)
	c.mutex.Unlock()
	return ok && !item.expired(timeNow())
}

func (c *twoQueueCache) Ping() error {
	return nil
}

func (c *twoQueueCache) deleteExpired() {
	now := timeNow()
	for _, key := range c.cache.Keys() {
		c.mutex.Lock()
		if item, ok := c.cache.Peek(key); ok && item.expired(now) {
			c.cache.Remove(key)
		}
		c.mutex.Unlock()
	}
}
