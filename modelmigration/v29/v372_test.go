// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/require"
)

func TestAddIndexIssueDependencyDependencyID(t *testing.T) {
	type IssueDependency struct {
		ID           int64 `xorm:"pk autoincr"`
		UserID       int64 `xorm:"NOT NULL"`
		IssueID      int64 `xorm:"UNIQUE(issue_dependency) NOT NULL"`
		DependencyID int64 `xorm:"UNIQUE(issue_dependency) NOT NULL"`
	}

	x, cleanup := migrationtest.PrepareTestEnv(t, 0, new(IssueDependency))
	defer cleanup()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, AddIndexIssueDependencyDependencyID(t.Context(), x))

	tables, err := x.DBMetas()
	require.NoError(t, err)
	var indexCols [][]string
	for _, table := range tables {
		if table.Name == "issue_dependency" {
			for _, index := range table.Indexes {
				indexCols = append(indexCols, index.Cols)
			}
		}
	}
	require.ElementsMatch(t, [][]string{{"issue_id", "dependency_id"}, {"dependency_id"}}, indexCols)
}
