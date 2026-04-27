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
	// Pre-migration shape of project_board (only the column we care about plus the
	// minimum needed to pass NOT NULL constraints during INSERT).
	type projectBoard struct {
		ID        int64 `xorm:"pk autoincr"`
		Title     string
		Sorting   int8  `xorm:"NOT NULL DEFAULT 0"`
		ProjectID int64 `xorm:"INDEX NOT NULL"`
		CreatorID int64 `xorm:"NOT NULL"`
	}

	x, deferrable := base.PrepareTestEnv(t, 0, new(projectBoard))
	defer deferrable()

	// Seed two rows: one at the int8 lower bound and one at the upper bound,
	// proving the migration preserves edge values without truncation.
	_, err := x.Insert(
		&projectBoard{Title: "first", Sorting: 0, ProjectID: 1, CreatorID: 1},
		&projectBoard{Title: "boundary", Sorting: 127, ProjectID: 1, CreatorID: 1},
	)
	require.NoError(t, err)

	require.NoError(t, WidenProjectBoardSorting(x))

	// Verify column type widened (skipped on SQLite where the migration is a no-op).
	if !setting.Database.Type.IsSQLite3() {
		table := base.LoadTableSchemasMap(t, x)["project_board"]
		require.NotNil(t, table)
		col := table.GetColumn("sorting")
		require.NotNil(t, col)
		// Each dialect spells INT differently; verify the type is one of the wider
		// names rather than TINYINT/INT2.
		assert.Contains(t,
			[]string{"INT", "INTEGER", "INT4"},
			col.SQLType.Name,
			"sorting column should have widened to int",
		)
		assert.False(t, col.Nullable, "sorting column should remain NOT NULL")
		assert.Equal(t, "0", col.Default, "sorting column should keep DEFAULT 0")
	}

	// Existing rows must be preserved verbatim.
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

	// Inserting a value > 127 must succeed after widening (would have failed with
	// TINYINT/INT2 either by truncation or out-of-range error).
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
	assert.Equal(t, 30000, got.Sorting, "value should round-trip without truncation")
}
