// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

type GetJSONError struct {
	err         error
	cachedError string // Golang error can't be stored in cache, only the string message could be stored
}

func (e *GetJSONError) ToError() error {
	if e.err != nil {
		return e.err
	}
	return errors.New("cached error: " + e.cachedError)
}

type StringCache interface {
	Ping() error

	Get(key string) (string, bool)
	Put(key, value string, ttl int64) error // ttl in seconds, 0 never expires, negative removes the key
	Delete(key string) error
	IsExist(key string) bool

	PutJSON(key string, v any, ttl int64) error
	GetJSON(key string, ptr any) (exist bool, err *GetJSONError)
}

type backend interface {
	Get(key string) (string, bool)
	Put(key, value string, ttl int64) error
	Delete(key string) error
	IsExist(key string) bool
	Ping() error
}

type stringCache struct {
	backend
}

func NewStringCache(cacheConfig setting.Cache) (StringCache, error) {
	cacheBackend, err := newBackend(cacheConfig)
	if err != nil {
		return nil, err
	}
	return &stringCache{backend: cacheBackend}, nil
}

func newBackend(cacheConfig setting.Cache) (backend, error) {
	gcInterval := time.Duration(util.IfZero(cacheConfig.Interval, 60)) * time.Second
	switch adapter := util.IfZero(cacheConfig.Adapter, "memory"); adapter {
	case "memory":
		// the old "memory" adapter doesn't have a limit, which can lead to OOM
		// now, use two-queue cache for in-memory cache with items limit, at most a few GB of memory will be used
		return newTwoQueueCache(strconv.FormatInt(10*1024*1024, 10), gcInterval)
	case "twoqueue":
		return newTwoQueueCache(cacheConfig.Conn, gcInterval)
	case "redis":
		return newRedisCache(cacheConfig.Conn), nil
	case "memcache":
		return newMemcacheCache(cacheConfig.Conn)
	default:
		return nil, fmt.Errorf("unknown cache adapter %q", adapter)
	}
}

func (sc *stringCache) Put(key, value string, ttl int64) error {
	if ttl < 0 {
		return sc.backend.Delete(key)
	}
	return sc.backend.Put(key, value, ttl)
}

const cachedErrorPrefix = "<CACHED-ERROR>:"

func (sc *stringCache) PutJSON(key string, v any, ttl int64) error {
	var s string
	switch v := v.(type) {
	case error:
		s = cachedErrorPrefix + v.Error()
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		s = util.UnsafeBytesToString(b)
	}
	return sc.Put(key, s, ttl)
}

func (sc *stringCache) GetJSON(key string, ptr any) (exist bool, getErr *GetJSONError) {
	s, ok := sc.Get(key)
	if !ok || s == "" {
		return false, nil
	}
	s, isCachedError := strings.CutPrefix(s, cachedErrorPrefix)
	if isCachedError {
		return true, &GetJSONError{cachedError: s}
	}
	if err := json.Unmarshal(util.UnsafeStringToBytes(s), ptr); err != nil {
		return false, &GetJSONError{err: err}
	}
	return true, nil
}
