// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestSessionRedisSharedConnFallback(t *testing.T) {
	tests := []struct {
		name        string
		iniStr      string
		wantContain string
		wantMissing string
	}{
		{
			name: "redis provider with empty PROVIDER_CONFIG falls back to shared [redis]",
			iniStr: `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
[session]
PROVIDER = redis
`,
			wantContain: "redis://127.0.0.1:6379/0",
		},
		{
			name: "session PROVIDER_CONFIG wins over shared [redis]",
			iniStr: `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
[session]
PROVIDER = redis
PROVIDER_CONFIG = redis://10.0.0.1:6379/1
`,
			wantContain: "redis://10.0.0.1:6379/1",
			wantMissing: "127.0.0.1",
		},
		{
			name: "no shared [redis]",
			iniStr: `
[session]
PROVIDER = redis
`,
			wantContain: "",
			wantMissing: "redis://",
		},
		{
			name: "file provider default path is untouched by shared [redis]",
			iniStr: `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
[session]
PROVIDER = file
`,
			wantContain: "sessions",
			wantMissing: "redis://",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer test.MockVariableValue(&Redis)()
			cfg, err := NewConfigProviderFromData(tt.iniStr)
			assert.NoError(t, err)

			loadRedisFrom(cfg)
			loadSessionFrom(cfg)
			// ProviderConfig is shadowed into a JSON blob at the end of loadSessionFrom
			assert.Contains(t, SessionConfig.ProviderConfig, tt.wantContain)
			if tt.wantMissing != "" {
				assert.NotContains(t, SessionConfig.ProviderConfig, tt.wantMissing)
			}
		})
	}
}
