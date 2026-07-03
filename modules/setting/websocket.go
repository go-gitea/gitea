// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "gitea.dev/modules/log"

const (
	PubsubTypeMemory = "memory"
	PubsubTypeRedis  = "redis"
)

// Websocket holds the settings for the websocket event delivery. The pubsub
// backend is scoped to websocket messages only, it is not a general-purpose
// pubsub service.
type WebsocketConfig struct {
	PubsubType    string
	PubsubConnStr string
}

var Websocket = WebsocketConfig{
	PubsubType: PubsubTypeMemory,
}

func loadWebsocketFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("websocket")
	Websocket.PubsubType = sec.Key("PUBSUB_TYPE").In(PubsubTypeMemory, []string{PubsubTypeMemory, PubsubTypeRedis})
	if Websocket.PubsubType == PubsubTypeRedis {
		Websocket.PubsubConnStr = sec.Key("PUBSUB_CONN_STR").String()
		if Websocket.PubsubConnStr == "" {
			log.Fatal("[websocket].PUBSUB_CONN_STR is required when PUBSUB_TYPE = redis")
		}
	}
}
