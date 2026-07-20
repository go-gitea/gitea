// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadRedisConfig(t *testing.T) {
	tests := []struct {
		name    string
		iniStr  string
		wantStr string
	}{
		{
			name:    "empty when [redis] absent",
			iniStr:  ``,
			wantStr: "",
		},
		{
			name: "loads CONN_STR",
			iniStr: `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
`,
			wantStr: "redis://127.0.0.1:6379/0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := NewConfigProviderFromData(tt.iniStr)
			assert.NoError(t, err)

			loadRedisFrom(cfg)
			t.Cleanup(func() { Redis.ConnStr = "" })
			assert.Equal(t, tt.wantStr, Redis.ConnStr)
		})
	}
}
