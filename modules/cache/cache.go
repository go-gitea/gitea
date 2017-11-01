// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cache

import (
	"code.gitea.io/gitea/modules/setting"

	mc "github.com/go-macaron/cache"
)

var conn mc.Cache

// NewContext start cache service
func NewContext() error {
	if setting.CacheService == nil || conn != nil {
		return nil
	}

	var err error
	conn, err = mc.NewCacher(setting.CacheService.Adapter, mc.Options{
		Adapter:       setting.CacheService.Adapter,
		AdapterConfig: setting.CacheService.Conn,
		Interval:      setting.CacheService.Interval,
	})
	return err
}

// GetInt returns key value from cache with callback when no key exists in cache
func GetInt(key string, getFunc func() (int, error)) (int, error) {
	if conn == nil || setting.CacheService.TTL == 0 {
		return getFunc()
	}
	if !conn.IsExist(key) {
		var (
			value int
			err   error
		)
		if value, err = getFunc(); err != nil {
			return value, err
		}
		conn.Put(key, value, int64(setting.CacheService.TTL.Seconds()))
	}
	return conn.Get(key).(int), nil
}

// GetInt64 returns key value from cache with callback when no key exists in cache
func GetInt64(key string, getFunc func() (int64, error)) (int64, error) {
	if conn == nil || setting.CacheService.TTL == 0 {
		return getFunc()
	}
	if !conn.IsExist(key) {
		var (
			value int64
			err   error
		)
		if value, err = getFunc(); err != nil {
			return value, err
		}
		conn.Put(key, value, int64(setting.CacheService.TTL.Seconds()))
	}
	return conn.Get(key).(int64), nil
}

// Remove key from cache
func Remove(key string) {
	if conn == nil {
		return
	}
	conn.Delete(key)
}
