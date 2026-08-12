// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"
)

func TestCacheRedisSharedConnFallback(t *testing.T) {
	defer test.MockVariableValue(&CacheService)()
	defer test.MockVariableValue(&Redis)()

	testConfigLoad(t, []any{loadRedisFrom, loadCacheFrom}, []configTestCase{
		{
			name: "redis adapter with empty HOST falls back to shared [redis]",
			ini:  "[redis]\nCONN_STR = redis://127.0.0.1:6379/0\n[cache]\nADAPTER = redis",
			want: []configCheck{field("HOST", &CacheService.Conn, "redis://127.0.0.1:6379/0")},
		},
		{
			name: "cache HOST wins over shared [redis]",
			ini:  "[redis]\nCONN_STR = redis://127.0.0.1:6379/0\n[cache]\nADAPTER = redis\nHOST = redis://10.0.0.1:6379/1",
			want: []configCheck{field("HOST", &CacheService.Conn, "redis://10.0.0.1:6379/1")},
		},
		{
			name: "no shared [redis] keeps previous behavior (empty conn)",
			ini:  "[cache]\nADAPTER = redis",
			want: []configCheck{field("HOST", &CacheService.Conn, "")},
		},
		{
			name: "memcache adapter is never affected by shared [redis]",
			ini:  "[redis]\nCONN_STR = redis://127.0.0.1:6379/0\n[cache]\nADAPTER = memcache",
			want: []configCheck{field("HOST", &CacheService.Conn, "")},
		},
	})
}
