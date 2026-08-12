// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"
)

func TestLoadWebsocketConfig(t *testing.T) {
	defer test.MockVariableValue(&Websocket)()
	defer test.MockVariableValue(&Redis)()

	testConfigLoad(t, []any{loadRedisFrom, loadWebsocketFrom}, []configTestCase{
		{
			name: "defaults to memory",
			want: []configCheck{
				field("PUBSUB_TYPE", &Websocket.PubsubType, PubsubTypeMemory),
				field("PUBSUB_CONN_STR", &Websocket.PubsubConnStr, ""),
			},
		},
		{
			name: "redis with its own conn str",
			ini:  "[websocket]\nPUBSUB_TYPE = redis\nPUBSUB_CONN_STR = redis://127.0.0.1:6379/0",
			want: []configCheck{
				field("PUBSUB_TYPE", &Websocket.PubsubType, PubsubTypeRedis),
				field("PUBSUB_CONN_STR", &Websocket.PubsubConnStr, "redis://127.0.0.1:6379/0"),
			},
		},
		{
			name: "redis falls back to the shared [redis] section",
			ini:  "[redis]\nCONN_STR = redis://127.0.0.1:6379/0\n[websocket]\nPUBSUB_TYPE = redis",
			want: []configCheck{
				field("PUBSUB_TYPE", &Websocket.PubsubType, PubsubTypeRedis),
				field("PUBSUB_CONN_STR", &Websocket.PubsubConnStr, "redis://127.0.0.1:6379/0"),
			},
		},
		{
			name: "own conn str wins over the shared one",
			ini:  "[redis]\nCONN_STR = redis://127.0.0.1:6379/0\n[websocket]\nPUBSUB_TYPE = redis\nPUBSUB_CONN_STR = redis://10.0.0.1:6379/1",
			want: []configCheck{
				field("PUBSUB_TYPE", &Websocket.PubsubType, PubsubTypeRedis),
				field("PUBSUB_CONN_STR", &Websocket.PubsubConnStr, "redis://10.0.0.1:6379/1"),
			},
		},
		{
			name: "memory ignores the shared [redis] section",
			ini:  "[redis]\nCONN_STR = redis://127.0.0.1:6379/0\n[websocket]\nPUBSUB_TYPE = memory",
			want: []configCheck{
				field("PUBSUB_TYPE", &Websocket.PubsubType, PubsubTypeMemory),
				field("PUBSUB_CONN_STR", &Websocket.PubsubConnStr, ""),
			},
		},
	})
}
