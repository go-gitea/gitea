// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadGlobalLockConfig(t *testing.T) {
	t.Run("DefaultGlobalLockConfig", func(t *testing.T) {
		iniStr := ``
		cfg, err := NewConfigProviderFromData(iniStr)
		assert.NoError(t, err)

		loadGlobalLockFrom(cfg)
		assert.EqualValues(t, "memory", GlobalLock.ServiceType)
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
		assert.EqualValues(t, "redis", GlobalLock.ServiceType)
		assert.EqualValues(t, "addrs=127.0.0.1:6379 db=0", GlobalLock.ServiceConnStr)
	})
}
