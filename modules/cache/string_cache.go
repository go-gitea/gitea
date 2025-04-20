// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"errors"
	"strings"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	chi_cache "gitea.com/go-chi/cache" //nolint:depguard
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
	Put(key, value string, ttl int64) error
	Delete(key string) error
	IsExist(key string) bool

	PutJSON(key string, v any, ttl int64) error
	GetJSON(key string, ptr any) (exist bool, err *GetJSONError)

	ChiCache() chi_cache.Cache
}

type stringCache struct {
	chiCache chi_cache.Cache
}

func NewStringCache(cacheConfig setting.Cache) (StringCache, error) {
	adapter := util.IfZero(cacheConfig.Adapter, "memory")
	interval := util.IfZero(cacheConfig.Interval, 60)
	cc, err := chi_cache.NewCacher(chi_cache.Options{
		Adapter:       adapter,
		AdapterConfig: cacheConfig.Conn,
		Interval:      interval,
	})
	if err != nil {
		return nil, err
	}
	return &stringCache{chiCache: cc}, nil
}

func (sc *stringCache) Ping() error {
	return sc.chiCache.Ping()
}

func (sc *stringCache) Get(key string) (string, bool) {
	v := sc.chiCache.Get(key)
	if v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func (sc *stringCache) Put(key, value string, ttl int64) error {
	return sc.chiCache.Put(key, value, ttl)
}

func (sc *stringCache) Delete(key string) error {
	return sc.chiCache.Delete(key)
}

func (sc *stringCache) IsExist(key string) bool {
	return sc.chiCache.IsExist(key)
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
	return sc.chiCache.Put(key, s, ttl)
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

func (sc *stringCache) ChiCache() chi_cache.Cache {
	return sc.chiCache
}
