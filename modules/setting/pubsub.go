// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "code.gitea.io/gitea/modules/log"

const (
	PubsubTypeMemory = "memory"
	PubsubTypeRedis  = "redis"
)

type PubsubConfig struct {
	Type    string
	ConnStr string
}

var Pubsub = PubsubConfig{
	Type: PubsubTypeMemory,
}

func loadPubsubFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("pubsub")
	Pubsub.Type = sec.Key("TYPE").In(PubsubTypeMemory, []string{PubsubTypeMemory, PubsubTypeRedis})
	if Pubsub.Type == PubsubTypeRedis {
		Pubsub.ConnStr = sec.Key("CONN_STR").String()
		if Pubsub.ConnStr == "" {
			log.Fatal("[pubsub].CONN_STR is required when TYPE = redis")
		}
	}
}
