// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ExtendCommentTreePathLength(t *testing.T) {
	if setting.Database.Type.IsSQLite3() {
		t.Skip("For SQLITE, varchar or char will always be represented as TEXT")
	}

	type Comment struct {
		ID       int64  `xorm:"pk autoincr"`
		TreePath string `xorm:"VARCHAR(255)"`
	}

	x, deferrable := base.PrepareTestEnv(t, 0, new(Comment))
	defer deferrable()

	require.NoError(t, ExtendCommentTreePathLength(x))

	table, err := x.TableInfo(new(Comment))
	require.NoError(t, err)

	column := table.GetColumn("tree_path")
	require.NotNil(t, column)

	assert.Contains(t, []string{"NVARCHAR", "VARCHAR"}, column.SQLType.Name)
	assert.EqualValues(t, 4000, column.Length)
}
