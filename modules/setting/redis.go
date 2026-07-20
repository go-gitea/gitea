// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"gitea.dev/modules/log"
	"gitea.dev/modules/nosql"
)

// Redis represents the shared redis configuration. Its connection string is used
// as the default for every redis-backed subsystem (cache, session, queue, global
// lock) whose own connection string is empty. A per-subsystem value always wins.
var Redis = struct {
	ConnStr string
}{}

func loadRedisFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("redis")
	Redis.ConnStr = sec.Key("CONN_STR").String()
	// validate only when set, so an absent [redis] section stays a no-op
	if Redis.ConnStr != "" && nosql.ToRedisURI(Redis.ConnStr) == nil {
		log.Fatal("CONN_STR %s is not a valid redis connection string", Redis.ConnStr)
	}
}
