// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"errors"
	"testing"
	"time"

	"gitea.dev/modules/nosql"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	_, err = NewStringCache(setting.Cache{Adapter: "memcache"})
	assert.ErrorContains(t, err, "requires [cache] HOST")
}

func TestStringCacheAdapters(t *testing.T) {
	now := time.Now()
	defer test.MockVariableValue(&timeNow, func() time.Time { return now })()
	assertClockExpiry := func(t *testing.T, cache StringCache, _ string) {
		require.NoError(t, cache.Put("expiring", "value", 10))
		now = now.Add(9 * time.Second)
		assert.True(t, cache.IsExist("expiring"))
		now = now.Add(time.Second)
		assert.False(t, cache.IsExist("expiring"))
		_, ok := cache.Get("expiring")
		assert.False(t, ok)
		require.NoError(t, cache.Put("expiring", "renewed", 0))
		value, ok := cache.Get("expiring")
		assert.True(t, ok)
		assert.Equal(t, "renewed", value)
	}
	assertRedisExpiry := func(t *testing.T, cache StringCache, conn string) {
		require.NoError(t, cache.Put("expiring", "value", 10))
		uri := nosql.ToRedisURI(conn)
		ttl := nosql.GetManager().GetRedisClient(uri.String()).TTL(t.Context(), uri.Query().Get("prefix")+"expiring").Val()
		assert.Positive(t, ttl)
		assert.LessOrEqual(t, ttl, 10*time.Second)
		require.NoError(t, cache.Delete("expiring"))
	}
	cases := []struct {
		adapter      string
		conns        func(t *testing.T) (string, string)
		assertExpiry func(t *testing.T, cache StringCache, conn string)
	}{
		{adapter: "memory", conns: func(*testing.T) (string, string) { return "", "" }, assertExpiry: assertClockExpiry},
		{adapter: "twoqueue", conns: func(*testing.T) (string, string) { return "100", `{"size":100}` }, assertExpiry: assertClockExpiry},
		{adapter: "redis", conns: func(t *testing.T) (string, string) {
			conn := test.PrepareTestRedis(t) + "?prefix=gitea-test-cache-"
			return conn + "first:", conn + "second:"
		}, assertExpiry: assertRedisExpiry},
	}
	for _, tc := range cases {
		t.Run(tc.adapter, func(t *testing.T) {
			firstConn, secondConn := tc.conns(t)
			first, err := NewStringCache(setting.Cache{Adapter: tc.adapter, Conn: firstConn, Interval: -1})
			require.NoError(t, err)
			second, err := NewStringCache(setting.Cache{Adapter: tc.adapter, Conn: secondConn, Interval: -1})
			require.NoError(t, err)
			require.NoError(t, first.Ping())

			require.NoError(t, first.Put("key", "value", 0))
			value, ok := first.Get("key")
			assert.True(t, ok)
			assert.Equal(t, "value", value)
			assert.True(t, first.IsExist("key"))
			assert.False(t, second.IsExist("key"))

			require.NoError(t, first.Delete("key"))
			_, ok = first.Get("key")
			assert.False(t, ok)
			assert.False(t, first.IsExist("key"))
			require.NoError(t, first.Delete("key"))

			require.NoError(t, first.Put("key", "value", 0))
			value, ok = first.GetAndDelete("key")
			assert.True(t, ok)
			assert.Equal(t, "value", value)
			_, ok = first.GetAndDelete("key")
			assert.False(t, ok)

			tc.assertExpiry(t, first, firstConn)
		})
	}
}

func TestNegativeItemTTLDisablesOnlyItemTTLCaching(t *testing.T) {
	createTestCache()
	defer test.MockVariableValue(&setting.CacheService.TTL, -1)()

	require.NoError(t, defaultCache.Put("key", "stale", 0))
	require.NoError(t, defaultCache.Put("key", "value", setting.CacheService.TTLSeconds()))
	assert.False(t, defaultCache.IsExist("key"))

	calls := 0
	for range 2 {
		data, err := GetString("key", func() (string, error) {
			calls++
			return "value", nil
		})
		assert.NoError(t, err)
		assert.Equal(t, "value", data)
	}
	assert.Equal(t, 2, calls)
	assert.False(t, defaultCache.IsExist("key"))

	require.NoError(t, defaultCache.Put("captcha", "value", 600))
	assert.True(t, defaultCache.IsExist("captcha"))
}

func TestMemcacheExpiration(t *testing.T) {
	defer test.MockVariableValue(&timeNow, func() time.Time { return time.Unix(1000, 0) })()
	assert.EqualValues(t, memcacheMaxRelativeTTL, memcacheExpiration(memcacheMaxRelativeTTL))
	assert.EqualValues(t, 1000+memcacheMaxRelativeTTL+1, memcacheExpiration(memcacheMaxRelativeTTL+1))
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
