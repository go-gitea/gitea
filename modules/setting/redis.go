// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

type RedisType struct {
	ConnStr string
}

// Redis represents the shared redis configuration. Its connection string is used
// as the default for every redis-backed subsystem (cache, session, queue, global
// lock) whose own connection string is empty. A per-subsystem value always wins.
var Redis *RedisType

func loadRedisFrom(rootCfg ConfigProvider) {
	Redis = &RedisType{} // make sure the Redis config is correctly loaded before it is accessed by other subsystems
	Redis.ConnStr = rootCfg.Section("redis").Key("CONN_STR").String()
}
