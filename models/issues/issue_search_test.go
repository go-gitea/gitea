// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"context"
	"strings"
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
)

func TestIssues_FilterWIPQueryPlan(t *testing.T) {
	if !setting.Database.Type.IsSQLite3() {
		t.Skip("SQLite query plan")
	}
	require.NoError(t, unittest.PrepareTestDatabase())
	var sql string
	var args []any
	require.NoError(t, db.WithTx(t.Context(), func(ctx context.Context) error {
		_, _, err := issues_model.IssueIDs(ctx, &issues_model.IssuesOptions{
			RepoIDs: []int64{1}, IsPull: optional.Some(true),
			IsClosed: optional.Some(false), IsWIP: optional.Some(true),
		})
		require.NoError(t, err)
		sess, ok := db.GetEngine(ctx).(*xorm.Session)
		require.True(t, ok)
		sql, args = sess.LastSQL()
		return nil
	}))
	rows, err := db.GetXORMEngineForTesting().DB().DB.QueryContext(t.Context(), "EXPLAIN QUERY PLAN "+sql, args...)
	require.NoError(t, err)
	defer rows.Close()
	var details []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		require.NoError(t, rows.Scan(&id, &parent, &unused, &detail))
		details = append(details, detail)
	}
	require.NoError(t, rows.Err())
	t.Log(strings.Join(details, "\n"))
	assert.Contains(t, strings.Join(details, "\n"), "is_wip=?", "WIP must be an index lookup, not a title scan")
}

func TestIssues_FilterWIP(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// fixture issue 2 is a pull in repo 1, mark it as WIP with a lowercase prefix on purpose
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	issue.Title = "[wip] issue2"
	require.NoError(t, issues_model.UpdateIssueCols(t.Context(), issue, "name"))

	base := &issues_model.IssuesOptions{
		RepoIDs: []int64{1},
		IsPull:  optional.Some(true),
	}

	only, err := issues_model.Issues(t.Context(), base.Copy(func(o *issues_model.IssuesOptions) {
		o.IsWIP = optional.Some(true)
	}))
	require.NoError(t, err)
	assert.NotEmpty(t, only)
	for _, issue := range only {
		assert.True(t, issues_model.HasWorkInProgressPrefix(issue.Title), issue.Title)
	}

	hidden, err := issues_model.Issues(t.Context(), base.Copy(func(o *issues_model.IssuesOptions) {
		o.IsWIP = optional.Some(false)
	}))
	require.NoError(t, err)
	for _, issue := range hidden {
		assert.False(t, issues_model.HasWorkInProgressPrefix(issue.Title), issue.Title)
	}

	all, err := issues_model.Issues(t.Context(), base)
	require.NoError(t, err)
	assert.Len(t, all, len(only)+len(hidden))
}
