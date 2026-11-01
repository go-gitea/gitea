// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddRunnerNameToActionTask(t *testing.T) {
	type ActionTask struct {
		ID       int64 `xorm:"pk"`
		RunnerID int64 `xorm:"index"`
	}
	type ActionRunner struct {
		ID      int64  `xorm:"pk"`
		Name    string `xorm:"VARCHAR(255)"`
		Deleted int64
	}

	x, deferrable := migrationtest.PrepareTestEnv(t, 0, new(ActionTask), new(ActionRunner))
	defer deferrable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(
		&ActionRunner{ID: 1, Name: "live-runner"},
		&ActionRunner{ID: 2, Name: "deleted-runner", Deleted: 1},

		&ActionTask{ID: 1, RunnerID: 1},
		&ActionTask{ID: 2, RunnerID: 2},
		&ActionTask{ID: 3, RunnerID: 3},
		&ActionTask{ID: 4, RunnerID: 0},
	)
	require.NoError(t, err)
	require.NoError(t, AddRunnerNameToActionTask(t.Context(), x))

	var names []string
	require.NoError(t, x.Table("action_task").Cols("runner_name").Asc("id").Find(&names))
	assert.Equal(t, []string{"live-runner", "deleted-runner", "", ""}, names)
}
