// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"fmt"
	"strconv"
	"time"

	"code.gitea.io/gitea/modules/setting"

	_ "gitea.com/go-chi/cache/memcache" //nolint:depguard // memcache plugin for cache, it is required for config "ADAPTER=memcache"
)

var defaultCache StringCache

// Init start cache service
func Init() error {
	if defaultCache == nil {
		c, err := NewStringCache(setting.CacheService.Cache)
		if err != nil {
			return err
		}
		for i := 0; i < 10; i++ {
			if err = c.Ping(); err == nil {
				break
			}
			time.Sleep(time.Second)
		}
		if err != nil {
			return err
		}
		defaultCache = c
	}
	return nil
}

const (
	testCacheKey = "DefaultCache.TestKey"
	// SlowCacheThreshold marks cache tests as slow
	// set to 30ms per discussion: https://github.com/go-gitea/gitea/issues/33190
	// TODO: Replace with metrics histogram
	SlowCacheThreshold = 30 * time.Millisecond
)

// Test performs delete, put and get operations on a predefined key
// returns
func Test() (time.Duration, error) {
	if defaultCache == nil {
		return 0, fmt.Errorf("default cache not initialized")
	}

	testData := fmt.Sprintf("%x", make([]byte, 500))

	start := time.Now()

	if err := defaultCache.Delete(testCacheKey); err != nil {
		return 0, fmt.Errorf("expect cache to delete data based on key if exist but got: %w", err)
	}
	if err := defaultCache.Put(testCacheKey, testData, 10); err != nil {
		return 0, fmt.Errorf("expect cache to store data but got: %w", err)
	}
	testVal, hit := defaultCache.Get(testCacheKey)
	if !hit {
		return 0, fmt.Errorf("expect cache hit but got none")
	}
	if testVal != testData {
		return 0, fmt.Errorf("expect cache to return same value as stored but got other")
	}

	return time.Since(start), nil
}

// GetCache returns the currently configured cache
func GetCache() StringCache {
	return defaultCache
}

// GetString returns the key value from cache with callback when no key exists in cache
func GetString(key string, getFunc func() (string, error)) (string, error) {
	if defaultCache == nil || setting.CacheService.TTL == 0 {
		return getFunc()
	}
	cached, exist := defaultCache.Get(key)
	if !exist {
		value, err := getFunc()
		if err != nil {
			return value, err
		}
		return value, defaultCache.Put(key, value, setting.CacheService.TTLSeconds())
	}
	return cached, nil
}

// GetInt64 returns key value from cache with callback when no key exists in cache
func GetInt64(key string, getFunc func() (int64, error)) (int64, error) {
	s, err := GetString(key, func() (string, error) {
		v, err := getFunc()
		return strconv.FormatInt(v, 10), err
	})
	if err != nil {
		return 0, err
	}
	if s == "" {
		return 0, nil
	}
	return strconv.ParseInt(s, 10, 64)
}

// Remove key from cache
func Remove(key string) {
	if defaultCache == nil {
		return
	}
	_ = defaultCache.Delete(key)
}
