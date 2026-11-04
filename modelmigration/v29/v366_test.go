// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

type issueBeforeV366 struct {
	ID     int64 `xorm:"pk autoincr"`
	RepoID int64 `xorm:"INDEX"`
}

func (issueBeforeV366) TableName() string { return "issue" }

func TestAddIsConfidentialToIssue(t *testing.T) {
	x, deferrable := migrationtest.PrepareTestEnv(t, 0, new(issueBeforeV366))
	defer deferrable()
	if x == nil || t.Failed() {
		return
	}

	const issueCount = 5
	for range issueCount {
		_, err := x.Insert(&issueBeforeV366{RepoID: 1})
		require.NoError(t, err)
	}

	require.NoError(t, AddIsConfidentialToIssue(t.Context(), x))
	require.NoError(t, AddIsConfidentialToIssue(t.Context(), x), "migration must be idempotent")

	type issueAfterV366 struct {
		ID             int64
		IsConfidential bool
	}
	var issues []issueAfterV366
	require.NoError(t, x.Table("issue").Asc("id").Find(&issues))
	require.Len(t, issues, issueCount)
	for _, issue := range issues {
		assert.False(t, issue.IsConfidential)
	}

	tables, err := x.DBMetas()
	require.NoError(t, err)
	for _, table := range tables {
		if table.Name != "issue" {
			continue
		}
		var index *schemas.Index
		for _, candidate := range table.Indexes {
			if candidate.XName(table.Name) == "IDX_issue_repo_confidential" {
				index = candidate
			}
		}
		require.NotNil(t, index)
		assert.Equal(t, []string{"repo_id", "is_confidential"}, index.Cols)
	}
}
