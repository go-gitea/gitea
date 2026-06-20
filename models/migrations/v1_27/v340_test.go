// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"testing"

	"gitea.dev/models/migrations/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_AddActionRunJobMatchingSchema(t *testing.T) {
	type ActionRunJob struct {
		ID      int64
		RunsOn  []string `xorm:"JSON TEXT"`
		Status  int      `xorm:"index"`
		Updated int64
	}
	type ActionRunJobLabel struct {
		ID    int64  `xorm:"pk autoincr"`
		JobID int64  `xorm:"UNIQUE(job_label) NOT NULL"`
		Label string `xorm:"UNIQUE(job_label) INDEX VARCHAR(255) NOT NULL"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(ActionRunJob))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	// statusWaiting=5, statusRunning=6, statusBlocked=7. Seed directly because the
	// migration package can't register the real ActionRunJob bean for YAML fixtures.
	for _, job := range []ActionRunJob{
		{ID: 1, RunsOn: []string{"ubuntu-latest", "self-hosted"}, Status: 5}, // backfilled
		{ID: 2, RunsOn: []string{"linux", "linux"}, Status: 5},               // backfilled (dedup)
		{ID: 3, RunsOn: nil, Status: 5},                                      // no rows (matches any runner)
		{ID: 4, RunsOn: []string{"macos"}, Status: 6},                        // running, not backfilled
		{ID: 5, RunsOn: []string{"windows"}, Status: 7},                      // blocked, becomes waiting later
	} {
		_, err := x.Insert(&job)
		require.NoError(t, err)
	}

	require.NoError(t, AddActionRunJobMatchingSchema(x))

	var labels []ActionRunJobLabel
	require.NoError(t, x.OrderBy("job_id, label").Find(&labels))

	got := make(map[int64][]string)
	for _, l := range labels {
		got[l.JobID] = append(got[l.JobID], l.Label)
	}
	assert.Equal(t, map[int64][]string{
		1: {"self-hosted", "ubuntu-latest"},
		2: {"linux"},
		5: {"windows"},
	}, got)
}
