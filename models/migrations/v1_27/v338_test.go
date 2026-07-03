// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"strings"
	"testing"

	"gitea.dev/models/migrations/migrationtest"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/require"
)

type issueBeforeLongTextMSSQLMigration struct {
	ID      int64  `xorm:"pk autoincr"`
	Content string `xorm:"VARCHAR(4000)"`
}

func (issueBeforeLongTextMSSQLMigration) TableName() string {
	return "issue"
}

type commentBeforeLongTextMSSQLMigration struct {
	ID      int64  `xorm:"pk autoincr"`
	Content string `xorm:"VARCHAR(4000)"`
	Patch   string `xorm:"VARCHAR(4000) patch"`
}

func (commentBeforeLongTextMSSQLMigration) TableName() string {
	return "comment"
}

func Test_ExpandIssueAndCommentLongTextFieldsForMSSQL(t *testing.T) {
	if !setting.Database.Type.IsMSSQL() {
		t.Skip("Only MSSQL needs to expand legacy nvarchar(4000) long-text columns")
	}

	x, deferrable := migrationtest.PrepareTestEnv(t, 0, new(issueBeforeLongTextMSSQLMigration), new(commentBeforeLongTextMSSQLMigration))
	defer deferrable()

	require.NoError(t, ExpandIssueAndCommentLongTextFieldsForMSSQL(x))
	require.NoError(t, ExpandIssueAndCommentLongTextFieldsForMSSQL(x))

	longText := strings.Repeat("x", 5000)
	_, err := x.Insert(&issueBeforeLongTextMSSQLMigration{Content: longText})
	require.NoError(t, err)

	_, err = x.Insert(&commentBeforeLongTextMSSQLMigration{Content: longText, Patch: longText})
	require.NoError(t, err)
}
