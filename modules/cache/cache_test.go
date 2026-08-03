// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"errors"
	"testing"
	"time"

	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
)

func createTestCache() {
	defaultCache, _ = NewStringCache(setting.Cache{
		Adapter: "memory",
		TTL:     time.Minute,
	})
	setting.CacheService.TTL = 24 * time.Hour
}

func TestNewContext(t *testing.T) {
	assert.NoError(t, Init())

	setting.CacheService.Cache = setting.Cache{Adapter: "redis", Conn: "some random string"}
	con, err := NewStringCache(setting.Cache{
		Adapter:  "rand",
		Conn:     "false conf",
		Interval: 100,
	})
	assert.Error(t, err)
	assert.Nil(t, con)
}

func TestTest(t *testing.T) {
	defaultCache = nil
	_, err := Test()
	assert.Error(t, err)

	createTestCache()
	elapsed, err := Test()
	assert.NoError(t, err)
	// mem cache should take from 300ns up to 1ms on modern hardware ...
	assert.Positive(t, elapsed)
	assert.Less(t, elapsed, SlowCacheThreshold)
}

func TestGetCache(t *testing.T) {
	createTestCache()

	assert.NotNil(t, GetCache())
}

func TestGetString(t *testing.T) {
	createTestCache()

	data, err := GetString("key", func() (string, error) {
		return "", errors.New("some error")
	})
	assert.Error(t, err)
	assert.Empty(t, data)

	data, err = GetString("key", func() (string, error) {
		return "", nil
	})
	assert.NoError(t, err)
	assert.Empty(t, data)

	data, err = GetString("key", func() (string, error) {
		return "some data", nil
	})
	assert.NoError(t, err)
	assert.Empty(t, data)
	Remove("key")

	data, err = GetString("key", func() (string, error) {
		return "some data", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "some data", data)

	data, err = GetString("key", func() (string, error) {
		return "", errors.New("some error")
	})
	assert.NoError(t, err)
	assert.Equal(t, "some data", data)
	Remove("key")
}

func TestGetInt64(t *testing.T) {
	createTestCache()

	data, err := GetInt64("key", func() (int64, error) {
		return 0, errors.New("some error")
	})
	assert.Error(t, err)
	assert.EqualValues(t, 0, data)

	data, err = GetInt64("key", func() (int64, error) {
		return 0, nil
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 0, data)

	data, err = GetInt64("key", func() (int64, error) {
		return 100, nil
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 0, data)
	Remove("key")

	data, err = GetInt64("key", func() (int64, error) {
		return 100, nil
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 100, data)

	data, err = GetInt64("key", func() (int64, error) {
		return 0, errors.New("some error")
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 100, data)
	Remove("key")
}

func TestSafeCacheKey(t *testing.T) {
	assert.Equal(t, "prefix:s-0~", safeCacheKey("prefix", "0~", 100))
	assert.Equal(t, "prefix:h-36a9e7f1c95b82ffb99743e0c5c4ce95d83c9a430aac59f84ef3cbfab6145068", safeCacheKey("prefix", " ", 100))

	assert.Equal(t, "prefix:s-a", safeCacheKey("prefix", "a", 10))
	assert.Equal(t, "prefix:h-961b6dd3ede3cb8ecbaacbd68de040cd78eb2ed5889130cceb4c49268ea4d506", safeCacheKey("prefix", "aa", 10))
}
