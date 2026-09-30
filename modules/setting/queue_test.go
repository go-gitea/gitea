// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestQueueRedisSharedConnFallback(t *testing.T) {
	tests := []struct {
		name      string
		iniStr    string
		queueName string
		wantConn  string
	}{
		{
			name: "redis queue with empty CONN_STR falls back to shared [redis]",
			iniStr: `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
[queue]
TYPE = redis
`,
			queueName: "test",
			wantConn:  "redis://127.0.0.1:6379/0",
		},
		{
			name: "base [queue] CONN_STR wins over shared [redis]",
			iniStr: `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
[queue]
TYPE = redis
CONN_STR = redis://10.0.0.1:6379/1
`,
			queueName: "test",
			wantConn:  "redis://10.0.0.1:6379/1",
		},
		{
			name: "per-queue CONN_STR wins over shared [redis]",
			iniStr: `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
[queue]
TYPE = redis
[queue.test]
CONN_STR = redis://10.0.0.1:6379/2
`,
			queueName: "test",
			wantConn:  "redis://10.0.0.1:6379/2",
		},
		{
			name: "other queues still fall back to shared [redis] when only one is overridden",
			iniStr: `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
[queue]
TYPE = redis
[queue.test]
CONN_STR = redis://10.0.0.1:6379/2
`,
			queueName: "other",
			wantConn:  "redis://127.0.0.1:6379/0",
		},
		{
			name: "no shared [redis] keeps previous behavior (built-in localhost default)",
			iniStr: `
[queue]
TYPE = redis
`,
			queueName: "test",
			wantConn:  "redis://127.0.0.1:6379/0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer test.MockVariableValue(&Redis)()
			cfg, err := NewConfigProviderFromData(tt.iniStr)
			assert.NoError(t, err)

			loadRedisFrom(cfg)
			q, err := GetQueueSettings(cfg, tt.queueName)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantConn, q.ConnStr)
		})
	}
}
