// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "code.gitea.io/gitea/modules/log"

// Sync represents configuration of sync
var Sync = struct {
	LockServiceType    string
	LockServiceConnStr string
}{
	LockServiceType: "memory",
}

func parseSyncSetting() {
	sec := Cfg.Section("sync")
	Sync.LockServiceType = sec.Key("LOCK_SERVICE_TYPE").MustString("memory")
	if Sync.LockServiceType != "memory" && Sync.LockServiceType != "redis" {
		log.Fatal("Unknown sync lock service type: %s", Sync.LockServiceType)
	}
	Sync.LockServiceConnStr = sec.Key("LOCK_SERVICE_CONN_STR").MustString("")
}
