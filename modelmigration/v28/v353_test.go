// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/require"
)

func TestAddWorkflowPathToActions(t *testing.T) {
	type ActionRun struct {
		ID int64 `xorm:"pk autoincr"`
	}
	type ActionSchedule struct {
		ID int64 `xorm:"pk autoincr"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(ActionRun), new(ActionSchedule))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}
	_, err := x.Insert(new(ActionRun), new(ActionSchedule))
	require.NoError(t, err)
	require.NoError(t, AddWorkflowPathToActions(t.Context(), x))

	for _, table := range []string{"action_run", "action_schedule"} {
		var workflowPath string
		has, err := x.SQL("SELECT workflow_path FROM "+table+" WHERE id = ?", 1).Get(&workflowPath)
		require.NoError(t, err)
		require.True(t, has)
		require.Empty(t, workflowPath)
	}
}
