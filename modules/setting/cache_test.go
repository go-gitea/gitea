// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCacheRedisSharedConnFallback(t *testing.T) {
	tests := []struct {
		name     string
		iniStr   string
		wantConn string
	}{
		{
			name: "redis adapter with empty HOST falls back to shared [redis]",
			iniStr: `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
[cache]
ADAPTER = redis
`,
			wantConn: "redis://127.0.0.1:6379/0",
		},
		{
			name: "cache HOST wins over shared [redis]",
			iniStr: `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
[cache]
ADAPTER = redis
HOST = redis://10.0.0.1:6379/1
`,
			wantConn: "redis://10.0.0.1:6379/1",
		},
		{
			name: "no shared [redis] keeps previous behavior (empty conn)",
			iniStr: `
[cache]
ADAPTER = redis
`,
			wantConn: "",
		},
		{
			name: "memcache adapter is never affected by shared [redis]",
			iniStr: `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
[cache]
ADAPTER = memcache
`,
			wantConn: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := NewConfigProviderFromData(tt.iniStr)
			assert.NoError(t, err)

			loadRedisFrom(cfg)
			loadCacheFrom(cfg)
			assert.Equal(t, tt.wantConn, CacheService.Conn)
		})
	}
}
