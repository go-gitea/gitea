// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"fmt"
	"strconv"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// ConvertDatabaseTable converts database and tables from utf8 to utf8mb4 if it's mysql and set ROW_FORMAT=dynamic
func ConvertDatabaseTable() error {
	if xormEngine.Dialect().URI().DBType != schemas.MYSQL {
		return nil
	}

	r, err := CheckCollations(xormEngine)
	if err != nil {
		return err
	}

	_, err = xormEngine.Exec(fmt.Sprintf("ALTER DATABASE `%s` CHARACTER SET utf8mb4 COLLATE %s", setting.Database.Name, r.ExpectedCollation))
	if err != nil {
		return err
	}

	tables, err := xormEngine.DBMetas()
	if err != nil {
		return err
	}
	for _, table := range tables {
		if _, err := xormEngine.Exec(fmt.Sprintf("ALTER TABLE `%s` ROW_FORMAT=dynamic", table.Name)); err != nil {
			return err
		}

		if _, err := xormEngine.Exec(fmt.Sprintf("ALTER TABLE `%s` CONVERT TO CHARACTER SET utf8mb4 COLLATE %s", table.Name, r.ExpectedCollation)); err != nil {
			return err
		}
	}

	return nil
}

// ConvertVarcharToNVarchar converts database and tables from varchar to nvarchar if it's mssql
func ConvertVarcharToNVarchar() error {
	if xormEngine.Dialect().URI().DBType != schemas.MSSQL {
		return nil
	}

	sess := xormEngine.NewSession()
	defer sess.Close()
	res, err := sess.QuerySliceString(`SELECT 'ALTER TABLE ' + OBJECT_NAME(SC.object_id) + ' MODIFY SC.name NVARCHAR(' + CONVERT(VARCHAR(5),SC.max_length) + ')'
FROM SYS.columns SC
JOIN SYS.types ST
ON SC.system_type_id = ST.system_type_id
AND SC.user_type_id = ST.user_type_id
WHERE ST.name ='varchar'`)
	if err != nil {
		return err
	}
	for _, row := range res {
		if len(row) == 1 {
			if _, err = sess.Exec(row[0]); err != nil {
				return err
			}
		}
	}
	return err
}

// Cell2Int64 converts a xorm.Cell type to int64,
// and handles possible irregular cases.
func Cell2Int64(val xorm.Cell) int64 {
	switch (*val).(type) {
	case []uint8:
		log.Trace("Cell2Int64 ([]uint8): %v", *val)

		v, _ := strconv.ParseInt(string((*val).([]uint8)), 10, 64)
		return v
	}
	return (*val).(int64)
}
