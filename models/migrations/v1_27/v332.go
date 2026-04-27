// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"code.gitea.io/gitea/models/migrations/base"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// WidenProjectBoardSorting changes project_board.sorting from int8 (TINYINT) to int (INTEGER)
// so the public API can expose a regular int and lift the 127 column upper bound.
func WidenProjectBoardSorting(x *xorm.Engine) error {
	if x.Dialect().URI().DBType == schemas.SQLITE {
		return nil
	}
	return base.ModifyColumn(x, "project_board", &schemas.Column{
		Name:           "sorting",
		SQLType:        schemas.SQLType{Name: "INT"},
		Nullable:       false,
		Default:        "0",
		DefaultIsEmpty: false,
	})
}
