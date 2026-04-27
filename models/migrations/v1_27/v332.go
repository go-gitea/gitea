// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// WidenProjectBoardSorting changes project_board.sorting from int8 to int so the
// API can stop truncating sort values and the column count is no longer capped at
// 127. DefaultIsEmpty: true is required because MSSQL's ALTER COLUMN rejects an
// inline DEFAULT, and MySQL's MODIFY COLUMN drops any DEFAULT not restated, so the
// default is reapplied for MySQL afterwards. Postgres and MSSQL keep the existing
// DEFAULT constraint independently of the type change.
func WidenProjectBoardSorting(x *xorm.Engine) error {
	// SQLite uses type affinity rather than strict types: a column declared TINYINT
	// already stores any 64-bit int, so the widening is a no-op. Updating the
	// declared type would require recreating the table (no ALTER COLUMN in SQLite)
	// for no behavioral gain.
	if setting.Database.Type.IsSQLite3() {
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
	if setting.Database.Type.IsMySQL() {
		if _, err := x.Exec("ALTER TABLE `project_board` ALTER `sorting` SET DEFAULT 0"); err != nil {
			return err
		}
	}
	return nil
}
