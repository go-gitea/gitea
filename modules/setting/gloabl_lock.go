// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/nosql"
	"code.gitea.io/gitea/modules/util"
)

// GlobalLock represents configuration of global lock
var GlobalLock = struct {
	ServiceType    string
	ServiceConnStr util.SensitiveURLString
}{
	ServiceType: "memory",
}

func loadGlobalLockFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("global_lock")
	GlobalLock.ServiceType = sec.Key("SERVICE_TYPE").MustString("memory")
	switch GlobalLock.ServiceType {
	case "memory":
	case "redis":
		connStr := sec.Key("SERVICE_CONN_STR").String()
		if connStr == "" {
			log.Fatal("SERVICE_CONN_STR is empty for redis")
		}
		u := nosql.ToRedisURI(connStr)
		if u == nil {
			log.Fatal("SERVICE_CONN_STR %s is not a valid redis connection string", connStr)
		}
		GlobalLock.ServiceConnStr = util.SensitiveURLString(connStr)
	default:
		log.Fatal("Unknown sync lock service type: %s", GlobalLock.ServiceType)
	}
}
