// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"strings"
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangeActionsRawConcurrencyToText(t *testing.T) {
	if !setting.Database.Type.IsMSSQL() {
		t.Skip("Only MSSQL needs a migration to widen raw_concurrency")
	}

	type ActionRun struct {
		ID             int64 `xorm:"pk autoincr"`
		RawConcurrency string
	}
	type ActionRunJob struct {
		ID             int64 `xorm:"pk autoincr"`
		RawConcurrency string
	}

	x, deferrable := migrationtest.PrepareTestEnv(t, 0, new(ActionRun), new(ActionRunJob))
	defer deferrable()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, ChangeActionsRawConcurrencyToText(t.Context(), x))

	long := strings.Repeat("界", 1000)
	run := &ActionRun{RawConcurrency: long}
	job := &ActionRunJob{RawConcurrency: long}
	_, err := x.Insert(run, job)
	require.NoError(t, err)

	gotRun, gotJob := &ActionRun{ID: run.ID}, &ActionRunJob{ID: job.ID}
	_, err = x.Get(gotRun)
	require.NoError(t, err)
	_, err = x.Get(gotJob)
	require.NoError(t, err)
	assert.Equal(t, long, gotRun.RawConcurrency)
	assert.Equal(t, long, gotJob.RawConcurrency)
}
