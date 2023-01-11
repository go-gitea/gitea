// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gopkg.in/ini.v1"
)

func TestParseSyncConfig(t *testing.T) {
	iniFile := ini.Empty()

	t.Run("RedisSyncConfig", func(t *testing.T) {
		iniFile.DeleteSection("sync")
		sec := iniFile.Section("sync")
		sec.NewKey("LOCK_SERVICE_TYPE", "redis")
		sec.NewKey("LOCK_SERVICE_CONN_STR", "addrs=127.0.0.1:6379 db=0")
		parseSyncConfig(iniFile)
	})
}
