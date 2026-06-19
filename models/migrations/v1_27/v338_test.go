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

	const statusWaiting = 5
	const statusRunning = 6
	const statusBlocked = 7

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(ActionRunJob))
	defer deferable()

	// waiting jobs: backfilled (with dedup of duplicate labels)
	_, err := x.Insert(&ActionRunJob{ID: 1, RunsOn: []string{"ubuntu-latest", "self-hosted"}, Status: statusWaiting})
	require.NoError(t, err)
	_, err = x.Insert(&ActionRunJob{ID: 2, RunsOn: []string{"linux", "linux"}, Status: statusWaiting})
	require.NoError(t, err)
	// waiting job with empty runs_on: no rows (matches any runner)
	_, err = x.Insert(&ActionRunJob{ID: 3, RunsOn: nil, Status: statusWaiting})
	require.NoError(t, err)
	// running job: not assigned again, so not backfilled
	_, err = x.Insert(&ActionRunJob{ID: 4, RunsOn: []string{"macos"}, Status: statusRunning})
	require.NoError(t, err)
	// blocked job: becomes waiting once its needs complete, so it must be backfilled
	_, err = x.Insert(&ActionRunJob{ID: 5, RunsOn: []string{"windows"}, Status: statusBlocked})
	require.NoError(t, err)

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
