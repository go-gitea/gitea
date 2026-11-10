// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/require"
)

func TestAddScopedWorkflowRepoFilter(t *testing.T) {
	type ActionScopedWorkflowSource struct {
		ID           int64 `xorm:"pk autoincr"`
		SourceRepoID int64
	}
	x, cleanup := migrationtest.PrepareTestEnv(t, 0, new(ActionScopedWorkflowSource))
	defer cleanup()
	if x == nil || t.Failed() {
		return
	}
	_, err := x.Insert(&ActionScopedWorkflowSource{SourceRepoID: 42})
	require.NoError(t, err)
	require.NoError(t, AddScopedWorkflowRepoFilter(t.Context(), x))
	var row struct {
		SourceRepoID int64
		RepoFilter   string
	}
	has, err := x.Table("action_scoped_workflow_source").Get(&row)
	require.NoError(t, err)
	require.True(t, has)
	require.EqualValues(t, 42, row.SourceRepoID)
	require.Empty(t, row.RepoFilter)
	require.NoError(t, AddScopedWorkflowRepoFilter(t.Context(), x))
}
