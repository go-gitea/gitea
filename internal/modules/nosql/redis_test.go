// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package nosql

import (
	"testing"
)

func TestToRedisURI(t *testing.T) {
	tests := []struct {
		name       string
		connection string
		want       string
	}{
		{
			name:       "old_default",
			connection: "addrs=127.0.0.1:6379 db=0",
			want:       "redis://127.0.0.1:6379/0",
		},
		{
			name:       "old_macaron_session_default",
			connection: "network=tcp,addr=127.0.0.1:6379,password=macaron,db=0,pool_size=100,idle_timeout=180",
			want:       "redis://:macaron@127.0.0.1:6379/0?idle_timeout=180s&pool_size=100",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToRedisURI(tt.connection); got == nil || got.String() != tt.want {
				t.Errorf(`ToRedisURI(%q) = %s, want %s`, tt.connection, got.String(), tt.want)
			}
		})
	}
}
