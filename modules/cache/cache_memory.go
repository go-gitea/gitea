// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"maps"
	"sync"
	"time"
)

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

type memoryCache struct {
	mutex sync.RWMutex
	items map[string]memoryItem
}

func newMemoryCache(gcInterval time.Duration) backend {
	cache := &memoryCache{items: map[string]memoryItem{}}
	startGC(gcInterval, cache.deleteExpired)
	return cache
}

func (c *memoryCache) Get(key string) (string, bool) {
	c.mutex.RLock()
	item, ok := c.items[key]
	c.mutex.RUnlock()
	if !ok || item.expired(timeNow()) {
		return "", false
	}
	return item.value, true
}

func (c *memoryCache) Put(key, value string, ttl int64) error {
	item := newMemoryItem(value, ttl)
	c.mutex.Lock()
	c.items[key] = item
	c.mutex.Unlock()
	return nil
}

func (c *memoryCache) Delete(key string) error {
	c.mutex.Lock()
	delete(c.items, key)
	c.mutex.Unlock()
	return nil
}

func (c *memoryCache) IsExist(key string) bool {
	_, ok := c.Get(key)
	return ok
}

func (c *memoryCache) Ping() error {
	return nil
}

func (c *memoryCache) deleteExpired() {
	now := timeNow()
	c.mutex.Lock()
	maps.DeleteFunc(c.items, func(_ string, item memoryItem) bool { return item.expired(now) })
	c.mutex.Unlock()
}
