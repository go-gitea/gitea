// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/setting"
	"github.com/stretchr/testify/assert"
)

func Test_ExtendCommentTreePathLength(t *testing.T) {
	if setting.Database.Type.IsSQLite3() {
		t.Skip("For SQLITE, varchar or char will always be represented as TEXT")
	}

	type Comment struct {
		ID       int64  `xorm:"pk autoincr"`
		TreePath string `xorm:"VARCHAR(255)"`
	}

	x, deferable := base.PrepareTestEnv(t, 0, new(Comment))
	defer deferable()

	assert.NoError(t, ExtendCommentTreePathLength(x))

	tables, err := x.DBMetas()
	assert.NoError(t, err)

	for _, table := range tables {
		switch table.Name {
		case "comment":
			column := table.GetColumn("tree_path")
			assert.NotNil(t, column)
			assert.Equal(t, "VARCHAR", column.SQLType.Name)
			assert.Equal(t, 4000, column.Length)
		}
	}
}
