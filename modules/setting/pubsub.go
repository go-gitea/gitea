// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "code.gitea.io/gitea/modules/log"

type PubsubConfig struct {
	Type    string
	ConnStr string
}

var Pubsub = PubsubConfig{
	Type: "memory",
}

func loadPubsubFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("pubsub")
	Pubsub.Type = sec.Key("TYPE").In("memory", []string{"memory", "redis"})
	if Pubsub.Type == "redis" {
		Pubsub.ConnStr = sec.Key("CONN_STR").String()
		if Pubsub.ConnStr == "" {
			log.Fatal("[pubsub].CONN_STR is required when TYPE = redis")
		}
	}
}
