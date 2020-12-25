// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func convertHookTaskTypeToVarcharAndTrim(x *xorm.Engine) error {
	type HookTask struct {
		Typ string `xorm:"VARCHAR(16) index notnull"`
	}

	alterSQL := x.Dialect().ModifyColumnSQL("hook_task", &schemas.Column{
		Name:      "typ",
		TableName: "hook_task",
		SQLType: schemas.SQLType{
			Name:          "VARCHAR",
			DefaultLength: 16,
		},
		Nullable: false,
	})
	if _, err := x.Exec(alterSQL); err != nil {
		return err
	}

	if _, err := x.Exec("UPDATE hook_task SET typ = TRIM(typ)"); err != nil {
		return err
	}

	_, err := x.Exec("UPDATE web_hook SET type = TRIM(type)")
	return err
}
