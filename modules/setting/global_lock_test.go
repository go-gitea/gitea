// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestLoadGlobalLockConfig(t *testing.T) {
	for _, tt := range []struct {
		name, config, serviceType, connStr string
	}{
		{"DefaultGlobalLockConfig", ``, "memory", ""},
		{"RedisGlobalLockConfig", `
[global_lock]
SERVICE_TYPE = redis
SERVICE_CONN_STR = addrs=127.0.0.1:6379 db=0
`, "redis", "addrs=127.0.0.1:6379 db=0"},
		{"RedisGlobalLockFallsBackToSharedRedis", `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
[global_lock]
SERVICE_TYPE = redis
`, "redis", "redis://127.0.0.1:6379/0"},
		{"RedisGlobalLockOwnConnWinsOverSharedRedis", `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
[global_lock]
SERVICE_TYPE = redis
SERVICE_CONN_STR = redis://10.0.0.1:6379/1
`, "redis", "redis://10.0.0.1:6379/1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			defer test.MockVariableValue(&Redis)()
			cfg, err := NewConfigProviderFromData(tt.config)
			assert.NoError(t, err)
			loadRedisFrom(cfg)
			loadGlobalLockFrom(cfg)
			assert.Equal(t, tt.serviceType, GlobalLock.ServiceType)
			if tt.serviceType == "redis" {
				assert.Equal(t, tt.connStr, GlobalLock.ServiceConnStr)
			}
		})
	}
}
