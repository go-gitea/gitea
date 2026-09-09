// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddPickupIndexToActionRunJob(t *testing.T) {
	type ActionRunJob struct {
		ID      int64 `xorm:"pk autoincr"`
		TaskID  int64
		Status  int
		Updated int64 `xorm:"updated"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(ActionRunJob))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&ActionRunJob{TaskID: 0, Status: 1})
	require.NoError(t, err)

	require.NoError(t, AddPickupIndexToActionRunJob(t.Context(), x))

	// the runner-poll query filters on (task_id, status) and sorts on updated, so all three columns
	// must land in the same index for the poll to stay index-ordered instead of filesorting.
	tables := migrationtest.LoadTableSchemasMap(t, x)
	table, ok := tables["action_run_job"]
	require.True(t, ok)
	var pickup *string
	for name, idx := range table.Indexes {
		if len(idx.Cols) == 3 {
			joined := name
			pickup = &joined
			assert.ElementsMatch(t, []string{"task_id", "status", "updated"}, idx.Cols,
				"the pickup index must cover the runner-poll predicate and sort keys")
		}
	}
	assert.NotNil(t, pickup, "expected a 3-column composite index covering task_id, status, updated")
}
