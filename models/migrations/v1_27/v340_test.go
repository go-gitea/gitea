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

	// Fixture data (loaded from fixtures/Test_AddActionRunJobMatchingSchema/action_run_job.yml):
	//   id=1 runs_on=["ubuntu-latest","self-hosted"] status=waiting  → backfilled (dedup of duplicate labels)
	//   id=2 runs_on=["linux","linux"]               status=waiting  → backfilled (dedup)
	//   id=3 runs_on=null                            status=waiting  → no rows (matches any runner)
	//   id=4 runs_on=["macos"]                       status=running  → not backfilled (already assigned)
	//   id=5 runs_on=["windows"]                     status=blocked  → backfilled (becomes waiting later)

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(ActionRunJob))
	defer deferable()
	if x == nil || t.Failed() {
		return
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
