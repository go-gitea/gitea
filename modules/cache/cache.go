// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"fmt"
	"strconv"

	"code.gitea.io/gitea/modules/setting"

	mc "gitea.com/go-chi/cache"

	_ "gitea.com/go-chi/cache/memcache" // memcache plugin for cache
)

var conn mc.Cache

func newCache(cacheConfig setting.Cache) (mc.Cache, error) {
	return mc.NewCacher(mc.Options{
		Adapter:       cacheConfig.Adapter,
		AdapterConfig: cacheConfig.Conn,
		Interval:      cacheConfig.Interval,
	})
}

// NewContext start cache service
func NewContext() error {
	var err error

	if conn == nil && setting.CacheService.Enabled {
		if conn, err = newCache(setting.CacheService.Cache); err != nil {
			return err
		}
		if err = conn.Ping(); err != nil {
			return err
		}
	}

	return err
}

// GetCache returns the currently configured cache
func GetCache() mc.Cache {
	return conn
}

// Get returns the key value from cache with callback when no key exists in cache
func Get[V interface{}](key string, getFunc func() (V, error)) (V, error) {
	if conn == nil || setting.CacheService.TTL == 0 {
		return getFunc()
	}

	cached := conn.Get(key)
	if value, ok := cached.(V); ok {
		return value, nil
	}

	value, err := getFunc()
	if err != nil {
		return value, err
	}

	return value, conn.Put(key, value, setting.CacheService.TTLSeconds())
}

// Set updates and returns the key value in the cache with callback. The old value is only removed if the updateFunc() is successful
func Set[V interface{}](key string, valueFunc func() (V, error)) (V, error) {
	if conn == nil || setting.CacheService.TTL == 0 {
		return valueFunc()
	}

	value, err := valueFunc()
	if err != nil {
		return value, err
	}

	return value, conn.Put(key, value, setting.CacheService.TTLSeconds())
}

// GetString returns the key value from cache with callback when no key exists in cache
func GetString(key string, getFunc func() (string, error)) (string, error) {
	if conn == nil || setting.CacheService.TTL == 0 {
		return getFunc()
	}

	cached := conn.Get(key)

	if cached == nil {
		value, err := getFunc()
		if err != nil {
			return value, err
		}
		return value, conn.Put(key, value, setting.CacheService.TTLSeconds())
	}

	if value, ok := cached.(string); ok {
		return value, nil
	}

	if stringer, ok := cached.(fmt.Stringer); ok {
		return stringer.String(), nil
	}

	return fmt.Sprintf("%s", cached), nil
}

// GetInt returns key value from cache with callback when no key exists in cache
func GetInt(key string, getFunc func() (int, error)) (int, error) {
	if conn == nil || setting.CacheService.TTL == 0 {
		return getFunc()
	}

	cached := conn.Get(key)

	if cached == nil {
		value, err := getFunc()
		if err != nil {
			return value, err
		}

		return value, conn.Put(key, value, setting.CacheService.TTLSeconds())
	}

	switch v := cached.(type) {
	case int:
		return v, nil
	case string:
		value, err := strconv.Atoi(v)
		if err != nil {
			return 0, err
		}
		return value, nil
	default:
		value, err := getFunc()
		if err != nil {
			return value, err
		}
		return value, conn.Put(key, value, setting.CacheService.TTLSeconds())
	}
}

// GetInt64 returns key value from cache with callback when no key exists in cache
func GetInt64(key string, getFunc func() (int64, error)) (int64, error) {
	if conn == nil || setting.CacheService.TTL == 0 {
		return getFunc()
	}

	cached := conn.Get(key)

	if cached == nil {
		value, err := getFunc()
		if err != nil {
			return value, err
		}

		return value, conn.Put(key, value, setting.CacheService.TTLSeconds())
	}

	switch v := conn.Get(key).(type) {
	case int64:
		return v, nil
	case string:
		value, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, err
		}
		return value, nil
	default:
		value, err := getFunc()
		if err != nil {
			return value, err
		}

		return value, conn.Put(key, value, setting.CacheService.TTLSeconds())
	}
}

// Remove key from cache
func Remove(key string) {
	if conn == nil {
		return
	}
	_ = conn.Delete(key)
}
