// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadSyncConfig(t *testing.T) {
	t.Run("DefaultSyncConfig", func(t *testing.T) {
		iniStr := ``
		cfg, err := NewConfigProviderFromData(iniStr)
		assert.NoError(t, err)

		loadSyncFrom(cfg)
		assert.EqualValues(t, "memory", Sync.LockServiceType)
	})

	t.Run("RedisSyncConfig", func(t *testing.T) {
		iniStr := `
[sync]
LOCK_SERVICE_TYPE = redis
LOCK_SERVICE_CONN_STR = addrs=127.0.0.1:6379 db=0
`
		cfg, err := NewConfigProviderFromData(iniStr)
		assert.NoError(t, err)

		loadSyncFrom(cfg)
		assert.EqualValues(t, "redis", Sync.LockServiceType)
		assert.EqualValues(t, "addrs=127.0.0.1:6379 db=0", Sync.LockServiceConnStr)
	})
}
