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

func TestCanRunJob(t *testing.T) {
	runner := &ActionRunner{AgentLabels: []string{"self-hosted", "Linux", "X64"}, Group: &ActionRunnerGroup{LowerName: "gpu"}}
	assert.True(t, runner.CanRunJob("gpu", []string{"SELF-HOSTED", "linux"}))
	assert.False(t, runner.CanRunJob("gpu", []string{"linux", "arm64"}))
	assert.True(t, runner.CanRunJob("gpu", nil))
	assert.False(t, runner.CanRunJob("", []string{"linux", "arm64"}))
	assert.False(t, runner.CanRunJob("", nil))
	assert.False(t, runner.CanRunJob("cpu", []string{"linux"}))
	assert.False(t, (&ActionRunner{AgentLabels: []string{"gpu"}}).CanRunJob("gpu", nil))
	assert.True(t, runner.CanRunJob("enterprise/gpu", nil))
	assert.False(t, runner.CanRunJob("org/gpu", nil))
	assert.False(t, runner.CanRunJob("other/gpu", nil))
	ownerRunner := &ActionRunner{Group: &ActionRunnerGroup{OwnerID: 3, LowerName: "gpu"}}
	assert.True(t, ownerRunner.CanRunJob("organization/gpu", nil))
	assert.False(t, ownerRunner.CanRunJob("ent/gpu", nil))
	assert.False(t, (&ActionRunner{Group: &ActionRunnerGroup{LowerName: "s"}}).CanRunJob("ſ", nil))
}
