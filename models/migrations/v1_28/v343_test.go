// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

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

	// statusWaiting=5, statusRunning=6, statusBlocked=7. Seed without explicit ids so
	// xorm lets the identity column assign them (MSSQL rejects explicit identity inserts),
	// and read the assigned ids back to key the expected labels.
	jobs := []*ActionRunJob{
		{RunsOn: []string{"ubuntu-latest", "self-hosted"}, Status: 5}, // backfilled
		{RunsOn: []string{"linux", "linux"}, Status: 5},               // backfilled (dedup)
		{RunsOn: nil, Status: 5},                                      // no rows (matches any runner)
		{RunsOn: []string{"macos"}, Status: 6},                        // running, not backfilled
		{RunsOn: []string{"windows"}, Status: 7},                      // blocked, becomes waiting later
	}
	for _, job := range jobs {
		_, err := x.Insert(job)
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
		jobs[0].ID: {"self-hosted", "ubuntu-latest"},
		jobs[1].ID: {"linux"},
		jobs[4].ID: {"windows"},
	}, got)
}
