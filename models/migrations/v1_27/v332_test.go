// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_WidenProjectBoardSorting(t *testing.T) {
	// Pre-migration shape: int8 sorting column.
	type projectBoard struct {
		ID        int64 `xorm:"pk autoincr"`
		Title     string
		Sorting   int8  `xorm:"NOT NULL DEFAULT 0"`
		ProjectID int64 `xorm:"INDEX NOT NULL"`
		CreatorID int64 `xorm:"NOT NULL"`
	}

	x, deferrable := base.PrepareTestEnv(t, 0, new(projectBoard))
	defer deferrable()

	_, err := x.Insert(
		&projectBoard{Title: "first", Sorting: 0, ProjectID: 1, CreatorID: 1},
		&projectBoard{Title: "boundary", Sorting: 127, ProjectID: 1, CreatorID: 1}, // int8 max
	)
	require.NoError(t, err)

	require.NoError(t, WidenProjectBoardSorting(x))

	// SQLite uses dynamic typing so the schema metadata still reports the original
	// declared type; only verify schema metadata on real RDBMSes.
	if !setting.Database.Type.IsSQLite3() {
		table := base.LoadTableSchemasMap(t, x)["project_board"]
		require.NotNil(t, table)
		col := table.GetColumn("sorting")
		require.NotNil(t, col)
		// MySQL and MSSQL report "INT", Postgres reports "INTEGER".
		assert.Contains(t, []string{"INT", "INTEGER"}, col.SQLType.Name)
		assert.False(t, col.Nullable)
		assert.Equal(t, "0", col.Default)
	}

	// Post-migration shape: same table, int sorting.
	type projectBoardWide struct {
		ID        int64 `xorm:"pk autoincr"`
		Title     string
		Sorting   int   `xorm:"NOT NULL DEFAULT 0"`
		ProjectID int64 `xorm:"INDEX NOT NULL"`
		CreatorID int64 `xorm:"NOT NULL"`
	}
	rows := make([]*projectBoardWide, 0, 2)
	require.NoError(t, x.Table("project_board").Asc("id").Find(&rows))
	require.Len(t, rows, 2)
	assert.Equal(t, 0, rows[0].Sorting)
	assert.Equal(t, 127, rows[1].Sorting)

	// Value well past int8 range — proves the column genuinely widened.
	_, err = x.Table("project_board").Insert(&projectBoardWide{
		Title:     "wide",
		Sorting:   30000,
		ProjectID: 1,
		CreatorID: 1,
	})
	require.NoError(t, err)

	var got projectBoardWide
	has, err := x.Table("project_board").Where("title=?", "wide").Get(&got)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, 30000, got.Sorting)
}
