// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"fmt"
	"strconv"
	"time"

	"gitea.dev/modules/json"

	lru "github.com/hashicorp/golang-lru/v2"
)

const twoQueueDefaultSize = 50000

type twoQueueCache struct {
	cache *lru.TwoQueueCache[string, memoryItem] // wrap the existing thread-safe 2Q (two-queue) cache directly
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
		return nil, fmt.Errorf("invalid two-queue cache HOST %q, expected a size or a JSON config", conn)
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

func (c *twoQueueCache) Put(key, value string, ttl int64) error {
	item := newMemoryItem(value, ttl)
	c.cache.Add(key, item)
	return nil
}

func (c *twoQueueCache) Delete(key string) error {
	c.cache.Remove(key)
	return nil
}

func (c *twoQueueCache) IsExist(key string) bool {
	item, ok := c.cache.Peek(key)
	return ok && !item.expired(timeNow())
}

func (c *twoQueueCache) Ping() error {
	return nil
}

func (c *twoQueueCache) deleteExpired() {
	now := timeNow()
	for _, key := range c.cache.Keys() {
		if item, ok := c.cache.Peek(key); ok && item.expired(now) {
			c.cache.Remove(key)
		}
	}
}

type memoryItem struct {
	value     string
	expiresAt time.Time
}

func newMemoryItem(value string, ttl int64) memoryItem {
	item := memoryItem{value: value}
	if ttl > 0 {
		item.expiresAt = timeNow().Add(time.Duration(ttl) * time.Second)
	}
	return item
}

func (item memoryItem) expired(now time.Time) bool {
	return !item.expiresAt.IsZero() && !now.Before(item.expiresAt)
}

func startGC(interval time.Duration, deleteExpired func()) {
	if interval <= 0 {
		return
	}
	go func() {
		for range time.Tick(interval) {
			deleteExpired()
		}
	}()
}
