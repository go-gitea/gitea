// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddActionTaskRunnerIDCreatedIndex(t *testing.T) {
	type ActionTask struct {
		ID       int64 `xorm:"pk autoincr"`
		RunnerID int64 `xorm:"index"`
		Status   int   `xorm:"index"`
		Created  int64
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(ActionTask))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, AddActionTaskRunnerIDCreatedIndex(t.Context(), x))

	indexes := migrationtest.LoadTableSchemasMap(t, x)["action_task"].Indexes
	require.Contains(t, indexes, "runner_id_created")
	assert.Equal(t, []string{"runner_id", "created"}, indexes["runner_id_created"].Cols)
	assert.NotContains(t, indexes, "runner_id")
	assert.Contains(t, indexes, "status")
}
