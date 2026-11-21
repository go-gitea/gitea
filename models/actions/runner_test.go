// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"
	"time"

	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestShouldPersistRunnerTimestamps(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		persist  func(timeutil.TimeStamp, time.Time) bool
		interval time.Duration
		freshAge time.Duration
	}{
		{"online", ShouldPersistLastOnline, RunnerHeartbeatInterval, 5 * time.Second},
		{"active", ShouldPersistLastActive, RunnerActiveInterval, time.Second},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			now := time.Now()
			for _, tt := range []struct {
				name string
				last timeutil.TimeStamp
				want bool
			}{
				{"fresh, skip write", timeutil.TimeStamp(now.Add(-scenario.freshAge).Unix()), false},
				{"exactly at interval, write", timeutil.TimeStamp(now.Add(-scenario.interval).Unix()), true},
				{"stale, write", timeutil.TimeStamp(now.Add(-2 * scenario.interval).Unix()), true},
				{"zero (never seen), write", 0, true},
			} {
				t.Run(tt.name, func(t *testing.T) {
					assert.Equal(t, tt.want, scenario.persist(tt.last, now))
				})
			}
		})
	}
}

func TestCanMatchLabelsCaseInsensitive(t *testing.T) {
	runner := &ActionRunner{AgentLabels: []string{"self-hosted", "Linux", "X64"}}
	assert.True(t, runner.CanMatchLabels([]string{"SELF-HOSTED", "linux"}))
	assert.False(t, runner.CanMatchLabels([]string{"linux", "arm64"}))
	assert.False(t, runner.CanMatchLabels(nil))
}
