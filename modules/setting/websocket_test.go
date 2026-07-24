// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestLoadWebsocketConfig(t *testing.T) {
	t.Run("DefaultWebsocketConfig", func(t *testing.T) {
		defer test.MockVariableValue(&Websocket)()
		defer test.MockVariableValue(&Redis)()
		iniStr := ``
		cfg, err := NewConfigProviderFromData(iniStr)
		assert.NoError(t, err)
		loadRedisFrom(cfg)
		loadWebsocketFrom(cfg)
		assert.Equal(t, PubsubTypeMemory, Websocket.PubsubType)
		assert.Empty(t, Websocket.PubsubConnStr)
	})

	t.Run("RedisWebsocketConfig", func(t *testing.T) {
		defer test.MockVariableValue(&Websocket)()
		defer test.MockVariableValue(&Redis)()
		iniStr := `
[websocket]
PUBSUB_TYPE = redis
PUBSUB_CONN_STR = redis://127.0.0.1:6379/0
`
		cfg, err := NewConfigProviderFromData(iniStr)
		assert.NoError(t, err)
		loadRedisFrom(cfg)
		loadWebsocketFrom(cfg)
		assert.Equal(t, PubsubTypeRedis, Websocket.PubsubType)
		assert.Equal(t, "redis://127.0.0.1:6379/0", Websocket.PubsubConnStr)
	})

	t.Run("RedisWebsocketFallsBackToSharedRedis", func(t *testing.T) {
		defer test.MockVariableValue(&Websocket)()
		defer test.MockVariableValue(&Redis)()
		iniStr := `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
[websocket]
PUBSUB_TYPE = redis
`
		cfg, err := NewConfigProviderFromData(iniStr)
		assert.NoError(t, err)

		loadRedisFrom(cfg)
		loadWebsocketFrom(cfg)
		assert.Equal(t, PubsubTypeRedis, Websocket.PubsubType)
		assert.Equal(t, "redis://127.0.0.1:6379/0", Websocket.PubsubConnStr)
	})

	t.Run("RedisWebsocketOwnConnWinsOverSharedRedis", func(t *testing.T) {
		defer test.MockVariableValue(&Websocket)()
		defer test.MockVariableValue(&Redis)()
		iniStr := `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
[websocket]
PUBSUB_TYPE = redis
PUBSUB_CONN_STR = redis://10.0.0.1:6379/1
`
		cfg, err := NewConfigProviderFromData(iniStr)
		assert.NoError(t, err)

		loadRedisFrom(cfg)
		loadWebsocketFrom(cfg)
		assert.Equal(t, PubsubTypeRedis, Websocket.PubsubType)
		assert.Equal(t, "redis://10.0.0.1:6379/1", Websocket.PubsubConnStr)
	})

	t.Run("MemoryWebsocketNeverAffectedBySharedRedis", func(t *testing.T) {
		defer test.MockVariableValue(&Websocket)()
		defer test.MockVariableValue(&Redis)()
		iniStr := `
[redis]
CONN_STR = redis://127.0.0.1:6379/0
[websocket]
PUBSUB_TYPE = memory
`
		cfg, err := NewConfigProviderFromData(iniStr)
		assert.NoError(t, err)

		loadRedisFrom(cfg)
		loadWebsocketFrom(cfg)
		assert.Equal(t, PubsubTypeMemory, Websocket.PubsubType)
		assert.Empty(t, Websocket.PubsubConnStr)
	})
}
