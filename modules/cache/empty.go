// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cache

import mc "gitea.com/go-chi/cache"

// noCache is the interface that operates the cache data.
type noCache struct{}

// newNoCache create a noop cache for chi
func newNoCache() (mc.Cache, error) {
	return &noCache{}, nil
}

// Put puts value into cache with key and expire time.
func (c noCache) Put(key string, val interface{}, timeout int64) error {
	return nil
}

// Get gets cached value by given key.
func (c noCache) Get(key string) interface{} {
	return ""
}

// Delete deletes cached value by given key.
func (c noCache) Delete(key string) error {
	return nil
}

// Incr increases cached int-type value by given key as a counter.
func (c noCache) Incr(key string) error {
	return nil
}

// Decr decreases cached int-type value by given key as a counter.
func (c noCache) Decr(key string) error {
	return nil
}

// IsExist returns true if cached value exists.
func (c noCache) IsExist(key string) bool {
	return false
}

// Flush deletes all cached data.
func (c noCache) Flush() error {
	return nil
}

// StartAndGC starts GC routine based on config string settings.
func (c noCache) StartAndGC(opt mc.Options) error {
	return nil
}

// Ping tests if the cache is alive.
func (c noCache) Ping() error {
	return nil
}
