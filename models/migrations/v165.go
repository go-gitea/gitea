// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"

	"code.gitea.io/gitea/modules/log"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func modifyColumn(x *xorm.Engine, tableName string, col *schemas.Column) error {
	var indexes map[string]*schemas.Index
	var err error
	// MSSQL have to remove index at first, otherwise alter column will fail
	// ref. https://sqlzealots.com/2018/05/09/error-message-the-index-is-dependent-on-column-alter-table-alter-column-failed-because-one-or-more-objects-access-this-column/
	if x.Dialect().URI().DBType == schemas.MSSQL {
		indexes, err = x.Dialect().GetIndexes(x.DB(), context.Background(), tableName)
		if err != nil {
			return err
		}

		for _, index := range indexes {
			_, err = x.Exec(x.Dialect().DropIndexSQL(tableName, index))
			if err != nil {
				return err
			}
		}
	}

	defer func() {
		for _, index := range indexes {
			_, err = x.Exec(x.Dialect().CreateIndexSQL(tableName, index))
			if err != nil {
				log.Error("Create index %s on table %s failed: %v", index.Name, tableName, err)
			}
		}
	}()

	alterSQL := x.Dialect().ModifyColumnSQL(tableName, col)
	if _, err := x.Exec(alterSQL); err != nil {
		return err
	}
	return nil
}

func convertHookTaskTypeToVarcharAndTrim(x *xorm.Engine) error {
	dbType := x.Dialect().URI().DBType
	if dbType == schemas.SQLITE { // For SQLITE, varchar or char will always be represented as TEXT
		return nil
	}

	type HookTask struct {
		Typ string `xorm:"VARCHAR(16) index"`
	}

	if err := modifyColumn(x, "hook_task", &schemas.Column{
		Name: "typ",
		SQLType: schemas.SQLType{
			Name: "VARCHAR",
		},
		Length:   16,
		Nullable: true, // To keep compatible as nullable
	}); err != nil {
		return err
	}

	var hookTaskTrimSQL string
	if dbType == schemas.MSSQL {
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

	if err := modifyColumn(x, "webhook", &schemas.Column{
		Name: "type",
		SQLType: schemas.SQLType{
			Name: "VARCHAR",
		},
		Length:   16,
		Nullable: true, // To keep compatible as nullable
	}); err != nil {
		return err
	}

	var webhookTrimSQL string
	if dbType == schemas.MSSQL {
		webhookTrimSQL = "UPDATE webhook SET type = RTRIM(LTRIM(type))"
	} else {
		webhookTrimSQL = "UPDATE webhook SET type = TRIM(type)"
	}
	_, err := x.Exec(webhookTrimSQL)
	return err
}
