// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestLoadGlobalLockConfig(t *testing.T) {
	t.Run("DefaultGlobalLockConfig", func(t *testing.T) {
		iniStr := ``
		cfg, err := NewConfigProviderFromData(iniStr)
		assert.NoError(t, err)

		loadGlobalLockFrom(cfg)
		assert.Equal(t, "memory", GlobalLock.ServiceType)
	})

	t.Run("RedisGlobalLockConfig", func(t *testing.T) {
		iniStr := `
[global_lock]
SERVICE_TYPE = redis
SERVICE_CONN_STR = addrs=127.0.0.1:6379 db=0
`
		cfg, err := NewConfigProviderFromData(iniStr)
		assert.NoError(t, err)

		loadGlobalLockFrom(cfg)
		assert.Equal(t, "redis", GlobalLock.ServiceType)
		assert.Equal(t, "addrs=127.0.0.1:6379 db=0", GlobalLock.ServiceConnStr)
	})

	t.Run("RedisGlobalLockFallsBackToSharedRedis", func(t *testing.T) {
		defer test.MockVariableValue(&Redis)()
		iniStr := `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
[global_lock]
SERVICE_TYPE = redis
`
		cfg, err := NewConfigProviderFromData(iniStr)
		assert.NoError(t, err)

		loadRedisFrom(cfg)
		loadGlobalLockFrom(cfg)
		assert.Equal(t, "redis", GlobalLock.ServiceType)
		assert.Equal(t, "redis://127.0.0.1:6379/0", GlobalLock.ServiceConnStr)
	})

	t.Run("RedisGlobalLockOwnConnWinsOverSharedRedis", func(t *testing.T) {
		defer test.MockVariableValue(&Redis)()
		iniStr := `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
[global_lock]
SERVICE_TYPE = redis
SERVICE_CONN_STR = redis://10.0.0.1:6379/1
`
		cfg, err := NewConfigProviderFromData(iniStr)
		assert.NoError(t, err)

		loadRedisFrom(cfg)
		loadGlobalLockFrom(cfg)
		assert.Equal(t, "redis", GlobalLock.ServiceType)
		assert.Equal(t, "redis://10.0.0.1:6379/1", GlobalLock.ServiceConnStr)
	})
}
