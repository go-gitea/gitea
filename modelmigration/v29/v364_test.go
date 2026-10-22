// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddIssueWIP(t *testing.T) {
	type Issue struct {
		ID       int64  `xorm:"pk autoincr"`
		RepoID   int64  `xorm:"INDEX"`
		IsClosed bool   `xorm:"INDEX"`
		IsPull   bool   `xorm:"INDEX"`
		Title    string `xorm:"name"`
	}
	x, cleanup := migrationtest.PrepareTestEnv(t, 0, new(Issue))
	defer cleanup()
	if x == nil || t.Failed() {
		return
	}
	_, err := x.Insert(&Issue{RepoID: 1, IsPull: true, Title: "[WIP] existing"})
	require.NoError(t, err)
	require.NoError(t, AddIssueWIP(t.Context(), x))
	require.NoError(t, AddIssueWIP(t.Context(), x))
	var rows []struct {
		Title string `xorm:"name"`
		IsWIP bool
	}
	require.NoError(t, x.Table("issue").Find(&rows))
	require.Len(t, rows, 1)
	assert.Equal(t, "[WIP] existing", rows[0].Title)
	assert.False(t, rows[0].IsWIP)
	indexes, err := x.Dialect().GetIndexes(x.DB(), t.Context(), "issue")
	require.NoError(t, err)
	require.Contains(t, indexes, "repo_wip")
	assert.Equal(t, []string{"repo_id", "is_closed", "is_pull", "is_wip"}, indexes["repo_wip"].Cols)
	assert.Contains(t, indexes, "repo_id", "existing indexes must survive migration")
}
