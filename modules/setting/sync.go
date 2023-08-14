// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/nosql"
)

// Sync represents configuration of sync
var Sync = struct {
	LockServiceType    string
	LockServiceConnStr string
}{
	LockServiceType: "memory",
}

func loadSyncFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("sync")
	Sync.LockServiceType = sec.Key("LOCK_SERVICE_TYPE").MustString("memory")
	switch Sync.LockServiceType {
	case "memory":
	case "redis":
		connStr := sec.Key("LOCK_SERVICE_CONN_STR").String()
		if connStr == "" {
			log.Fatal("LOCK_SERVICE_CONN_STR is empty for redis")
		}
		u := nosql.ToRedisURI(connStr)
		if u == nil {
			log.Fatal("LOCK_SERVICE_CONN_STR %s is not right for redis", connStr)
		}
		Sync.LockServiceConnStr = connStr
	default:
		log.Fatal("Unknown sync lock service type: %s", Sync.LockServiceType)
	}
}
