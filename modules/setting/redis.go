// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// Redis represents the shared redis configuration. Its connection string is used
// as the default for every redis-backed subsystem (cache, session, queue, global
// lock) whose own connection string is empty. A per-subsystem value always wins.
var Redis = struct {
	ConnStr string
}{}

func loadRedisFrom(rootCfg ConfigProvider) {
	Redis.ConnStr = rootCfg.Section("redis").Key("CONN_STR").String()
}
