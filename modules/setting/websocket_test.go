// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestLoadWebsocketConfig(t *testing.T) {
	cases := []struct {
		name     string
		ini      string
		wantType string
		wantConn string
	}{
		{
			name:     "defaults to memory",
			wantType: PubsubTypeMemory,
		},
		{
			name:     "redis with its own conn str",
			ini:      "[websocket]\nPUBSUB_TYPE = redis\nPUBSUB_CONN_STR = redis://127.0.0.1:6379/0",
			wantType: PubsubTypeRedis,
			wantConn: "redis://127.0.0.1:6379/0",
		},
		{
			name:     "redis falls back to the shared [redis] section",
			ini:      "[redis]\nCONN_STR = redis://127.0.0.1:6379/0\n[websocket]\nPUBSUB_TYPE = redis",
			wantType: PubsubTypeRedis,
			wantConn: "redis://127.0.0.1:6379/0",
		},
		{
			name:     "own conn str wins over the shared one",
			ini:      "[redis]\nCONN_STR = redis://127.0.0.1:6379/0\n[websocket]\nPUBSUB_TYPE = redis\nPUBSUB_CONN_STR = redis://10.0.0.1:6379/1",
			wantType: PubsubTypeRedis,
			wantConn: "redis://10.0.0.1:6379/1",
		},
		{
			name:     "memory ignores the shared [redis] section",
			ini:      "[redis]\nCONN_STR = redis://127.0.0.1:6379/0\n[websocket]\nPUBSUB_TYPE = memory",
			wantType: PubsubTypeMemory,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer test.MockVariableValue(&Websocket)()
			defer test.MockVariableValue(&Redis)()
			cfg, err := NewConfigProviderFromData(tc.ini)
			assert.NoError(t, err)

			loadRedisFrom(cfg)
			loadWebsocketFrom(cfg)
			assert.Equal(t, tc.wantType, Websocket.PubsubType)
			assert.Equal(t, tc.wantConn, Websocket.PubsubConnStr)
		})
	}
}
