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
		Typ string `xorm:"VARCHAR(16) index"`
	}

	alterSQL := x.Dialect().ModifyColumnSQL("hook_task", &schemas.Column{
		Name:      "typ",
		TableName: "hook_task",
		SQLType: schemas.SQLType{
			Name: "VARCHAR",
		},
		Length:   16,
		Nullable: true, // To keep compatible as nullable
	})
	if _, err := x.Exec(alterSQL); err != nil {
		return err
	}

	var hookTaskTrimSQL string
	if x.Dialect().URI().DBType == schemas.MSSQL {
		hookTaskTrimSQL = "UPDATE hook_task SET typ = RTRIM(LTRIM(typ))"
	} else {
		hookTaskTrimSQL = "UPDATE hook_task SET typ = TRIM(typ)"
	}
	if _, err := x.Exec(hookTaskTrimSQL); err != nil {
		return err
	}

	type Webhook struct {
		Type string `xorm:"VARCHAR(16) index"`
	}

	alterSQL = x.Dialect().ModifyColumnSQL("webhook", &schemas.Column{
		Name:      "type",
		TableName: "webhook",
		SQLType: schemas.SQLType{
			Name: "VARCHAR",
		},
		Length:   16,
		Nullable: true, // To keep compatible as nullable
	})
	if _, err := x.Exec(alterSQL); err != nil {
		return err
	}

	var webhookTrimSQL string
	if x.Dialect().URI().DBType == schemas.MSSQL {
		webhookTrimSQL = "UPDATE webhook SET type = RTRIM(LTRIM(type))"
	} else {
		webhookTrimSQL = "UPDATE webhook SET type = TRIM(type)"
	}
	_, err := x.Exec(webhookTrimSQL)
	return err
}
