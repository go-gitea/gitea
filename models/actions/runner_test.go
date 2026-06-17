// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"
	"time"

	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestShouldPersistLastOnline(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		last timeutil.TimeStamp
		want bool
	}{
		{
			name: "fresh, skip write",
			last: timeutil.TimeStamp(now.Add(-5 * time.Second).Unix()),
			want: false,
		},
		{
			name: "exactly at interval, write",
			last: timeutil.TimeStamp(now.Add(-RunnerHeartbeatInterval).Unix()),
			want: true,
		},
		{
			name: "stale, write",
			last: timeutil.TimeStamp(now.Add(-2 * RunnerHeartbeatInterval).Unix()),
			want: true,
		},
		{
			name: "zero (never seen), write",
			last: 0,
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ShouldPersistLastOnline(tt.last, now))
		})
	}
}
