// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddActionQueueIndexes(t *testing.T) {
	type ActionRunJob struct {
		ID      int64 `xorm:"pk autoincr"`
		RepoID  int64
		TaskID  int64
		Status  int
		Updated int64 `xorm:"updated"`
	}
	type ActionRun struct {
		ID     int64 `xorm:"pk autoincr"`
		RepoID int64
		Status int
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(ActionRunJob), new(ActionRun))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, AddActionQueueIndexes(t.Context(), x))

	tables := migrationtest.LoadTableSchemasMap(t, x)
	indexCols := func(table string) [][]string {
		schema, ok := tables[table]
		require.True(t, ok)
		var cols [][]string
		for _, idx := range schema.Indexes {
			cols = append(cols, idx.Cols)
		}
		return cols
	}

	assert.Contains(t, indexCols("action_run_job"), []string{"task_id", "status", "updated"})
	assert.Contains(t, indexCols("action_run_job"), []string{"repo_id", "status"})
	assert.Contains(t, indexCols("action_run"), []string{"repo_id", "status"})
}
