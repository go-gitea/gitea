// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type issueBeforeV362 struct {
	ID    int64 `xorm:"pk autoincr"`
	Title string
	Ref   string
}

func (issueBeforeV362) TableName() string { return "issue" }

func TestRemoveIssueRef(t *testing.T) {
	x, deferrable := migrationtest.PrepareTestEnv(t, 0, new(issueBeforeV362))
	defer deferrable()
	if x == nil || t.Failed() {
		return
	}

	issue := issueBeforeV362{Title: "Preserved issue", Ref: "refs/heads/main"}
	_, err := x.Insert(&issue)
	require.NoError(t, err)

	if setting.Database.Type.IsSQLite3() {
		_, err = x.Exec("CREATE VIEW issue_view AS SELECT id FROM issue")
		require.NoError(t, err)
		require.Error(t, RemoveIssueRef(t.Context(), x))
		var actual issueBeforeV362
		has, err := x.Get(&actual)
		require.NoError(t, err)
		require.True(t, has)
		assert.Equal(t, issue, actual)
		_, err = x.Exec("DROP VIEW issue_view")
		require.NoError(t, err)
	}

	require.NoError(t, RemoveIssueRef(t.Context(), x))
	require.NoError(t, RemoveIssueRef(t.Context(), x))
	assert.NotContains(t, migrationtest.LoadTableSchemasMap(t, x)["issue"].ColumnsSeq(), "ref")
	var actual issueBeforeV362
	has, err := x.Select("id, title").Get(&actual)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, issue.ID, actual.ID)
	assert.Equal(t, issue.Title, actual.Title)
}
