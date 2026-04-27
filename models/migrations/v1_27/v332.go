// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"code.gitea.io/gitea/models/migrations/base"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// WidenProjectBoardSorting changes project_board.sorting from int8 (TINYINT/SMALLINT)
// to int. The previous int8 type capped projects at 127 columns and forced the API
// to truncate user-supplied sort values. SQLite uses dynamic typing so the schema
// type is cosmetic; existing rows already store the wider value.
//
// `base.ModifyColumn` is called with DefaultIsEmpty: true because MSSQL's ALTER
// COLUMN syntax does not accept inline DEFAULT (it would error on the keyword).
// On MySQL, MODIFY COLUMN without an explicit DEFAULT drops the existing default,
// so it has to be reapplied. On Postgres and MSSQL the DEFAULT constraint is
// maintained separately from the column type and is preserved automatically.
func WidenProjectBoardSorting(x *xorm.Engine) error {
	if x.Dialect().URI().DBType == schemas.SQLITE {
		return nil
	}
	if err := base.ModifyColumn(x, "project_board", &schemas.Column{
		Name:           "sorting",
		SQLType:        schemas.SQLType{Name: "INT"},
		Nullable:       false,
		DefaultIsEmpty: true,
	}); err != nil {
		return err
	}
	if x.Dialect().URI().DBType == schemas.MYSQL {
		if _, err := x.Exec("ALTER TABLE `project_board` ALTER `sorting` SET DEFAULT 0"); err != nil {
			return err
		}
	}
	return nil
}
