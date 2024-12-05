// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"fmt"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"

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
	assert.Less(t, elapsed, time.Millisecond)
}

func TestGetCache(t *testing.T) {
	createTestCache()

	assert.NotNil(t, GetCache())
}

func TestGetString(t *testing.T) {
	createTestCache()

	data, err := GetString("key", func() (string, error) {
		return "", fmt.Errorf("some error")
	})
	assert.Error(t, err)
	assert.Equal(t, "", data)

	data, err = GetString("key", func() (string, error) {
		return "", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "", data)

	data, err = GetString("key", func() (string, error) {
		return "some data", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "", data)
	Remove("key")

	data, err = GetString("key", func() (string, error) {
		return "some data", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "some data", data)

	data, err = GetString("key", func() (string, error) {
		return "", fmt.Errorf("some error")
	})
	assert.NoError(t, err)
	assert.Equal(t, "some data", data)
	Remove("key")
}

func TestGetInt64(t *testing.T) {
	createTestCache()

	data, err := GetInt64("key", func() (int64, error) {
		return 0, fmt.Errorf("some error")
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
		return 0, fmt.Errorf("some error")
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 100, data)
	Remove("key")
}
