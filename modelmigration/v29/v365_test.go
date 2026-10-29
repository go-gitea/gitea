// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddActionTaskRunnerIDIDIndex(t *testing.T) {
	type ActionTask struct {
		ID       int64 `xorm:"pk autoincr"`
		RunnerID int64 `xorm:"index"`
		Status   int   `xorm:"index"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(ActionTask))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, AddActionTaskRunnerIDIDIndex(t.Context(), x))

	schema, ok := migrationtest.LoadTableSchemasMap(t, x)["action_task"]
	require.True(t, ok)
	var cols [][]string
	for _, idx := range schema.Indexes {
		cols = append(cols, idx.Cols)
	}
	assert.Contains(t, cols, []string{"runner_id", "id"})
	// existing indexes must survive the partial-struct sync
	assert.Contains(t, cols, []string{"runner_id"})
	assert.Contains(t, cols, []string{"status"})
}
