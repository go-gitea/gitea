// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoverActionRunJobPickupIndex(t *testing.T) {
	type ActionRunJob struct {
		ID               int64 `xorm:"pk autoincr"`
		RepoID           int64
		TaskID           int64 `xorm:"index(pickup)"`
		Status           int   `xorm:"index(pickup)"`
		IsReusableCaller bool
		Updated          int64 `xorm:"index(pickup)"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(ActionRunJob))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, CoverActionRunJobPickupIndex(t.Context(), x))

	indexes, err := x.Dialect().GetIndexes(x.DB(), t.Context(), "action_run_job")
	require.NoError(t, err)
	require.Contains(t, indexes, "pickup")
	assert.Equal(t, []string{"task_id", "status", "is_reusable_caller", "updated", "id", "repo_id"}, indexes["pickup"].Cols)
}
