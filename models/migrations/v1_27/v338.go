// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"fmt"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/models/migrations/base"

	"xorm.io/xorm/schemas"
)

type issueWithLongTextContent struct {
	Content string `xorm:"LONGTEXT"`
}

func (issueWithLongTextContent) TableName() string {
	return "issue"
}

type commentWithLongTextFields struct {
	Content     string `xorm:"LONGTEXT"`
	PatchQuoted string `xorm:"LONGTEXT patch"`
}

func (commentWithLongTextFields) TableName() string {
	return "comment"
}

func isMSSQLMaxTextColumn(column *schemas.Column) bool {
	if column.Length != -1 {
		return false
	}
	return strings.EqualFold(column.SQLType.Name, schemas.Varchar) || strings.EqualFold(column.SQLType.Name, schemas.NVarchar)
}

func modifyLongTextColumnsForMSSQL(x db.EngineMigration, bean any, columnNames ...string) error {
	table, err := x.TableInfo(bean)
	if err != nil {
		return err
	}

	for _, columnName := range columnNames {
		column := table.GetColumn(columnName)
		if column == nil {
			return fmt.Errorf("column %s does not exist in table %s", columnName, table.Name)
		}
		if isMSSQLMaxTextColumn(column) {
			continue
		}
		if err := base.ModifyColumn(x, table.Name, column); err != nil {
			return fmt.Errorf("modify %s.%s: %w", table.Name, columnName, err)
		}
	}

	return nil
}

// ExpandIssueAndCommentLongTextFieldsForMSSQL expands legacy MSSQL nvarchar(4000)
// columns to nvarchar(max) so PR push comments and long issue content are not truncated.
func ExpandIssueAndCommentLongTextFieldsForMSSQL(x db.EngineMigration) error {
	if x.Dialect().URI().DBType != schemas.MSSQL {
		return nil
	}

	if err := modifyLongTextColumnsForMSSQL(x, new(issueWithLongTextContent), "content"); err != nil {
		return err
	}
	return modifyLongTextColumnsForMSSQL(x, new(commentWithLongTextFields), "content", "patch")
}
