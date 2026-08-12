// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddQueueRankToActionRunJob(t *testing.T) {
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

	require.NoError(t, AddQueueRankToActionRunJob(t.Context(), x))

	// pre-existing rows must default to the natural FIFO position
	var queueRank int64
	has, err := x.SQL("SELECT queue_rank FROM action_run_job WHERE id = ?", 1).Get(&queueRank)
	require.NoError(t, err)
	require.True(t, has)
	require.EqualValues(t, 0, queueRank)

	// the runner-poll query filters on (task_id, status) and sorts on (queue_rank, updated), so all four
	// columns must land in the same index for the poll to stay index-ordered instead of filesorting.
	tables := migrationtest.LoadTableSchemasMap(t, x)
	table, ok := tables["action_run_job"]
	require.True(t, ok)
	var pickup *string
	for name, idx := range table.Indexes {
		if len(idx.Cols) == 4 {
			joined := name
			pickup = &joined
			assert.ElementsMatch(t, []string{"task_id", "status", "queue_rank", "updated"}, idx.Cols,
				"the pickup index must cover the runner-poll predicate and sort keys")
		}
	}
	assert.NotNil(t, pickup, "expected a 4-column composite index covering task_id, status, queue_rank, updated")
}
