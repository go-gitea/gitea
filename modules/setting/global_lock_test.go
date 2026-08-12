// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"
)

func TestLoadGlobalLockConfig(t *testing.T) {
	defer test.MockVariableValue(&GlobalLock)()
	defer test.MockVariableValue(&Redis)()

	testConfigLoad(t, []any{loadRedisFrom, loadGlobalLockFrom}, []configTestCase{
		{
			name: "defaults to memory",
			want: []configCheck{field("SERVICE_TYPE", &GlobalLock.ServiceType, "memory")},
		},
		{
			name: "redis with its own conn str",
			ini:  "[global_lock]\nSERVICE_TYPE = redis\nSERVICE_CONN_STR = addrs=127.0.0.1:6379 db=0",
			want: []configCheck{
				field("SERVICE_TYPE", &GlobalLock.ServiceType, "redis"),
				field("SERVICE_CONN_STR", &GlobalLock.ServiceConnStr, "addrs=127.0.0.1:6379 db=0"),
			},
		},
		{
			name: "redis falls back to the shared [redis] section",
			ini:  "[redis]\nCONN_STR = redis://127.0.0.1:6379/0\n[global_lock]\nSERVICE_TYPE = redis",
			want: []configCheck{
				field("SERVICE_TYPE", &GlobalLock.ServiceType, "redis"),
				field("SERVICE_CONN_STR", &GlobalLock.ServiceConnStr, "redis://127.0.0.1:6379/0"),
			},
		},
		{
			name: "own conn str wins over the shared one",
			ini:  "[redis]\nCONN_STR = redis://127.0.0.1:6379/0\n[global_lock]\nSERVICE_TYPE = redis\nSERVICE_CONN_STR = redis://10.0.0.1:6379/1",
			want: []configCheck{
				field("SERVICE_TYPE", &GlobalLock.ServiceType, "redis"),
				field("SERVICE_CONN_STR", &GlobalLock.ServiceConnStr, "redis://10.0.0.1:6379/1"),
			},
		},
	})
}
