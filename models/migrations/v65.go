// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
)

func mysqlColumnsToUTF8MB4(x *xorm.Engine) (err error) {
	if !setting.UseMySQL {
		log.Info("Nothing to do")
		return nil
	}

	const maxvc = 191
	migrationSuccess := true

	tables, err := x.DBMetas()
	if err != nil {
		return fmt.Errorf("cannot get tables: %v", err)
	}
	for _, table := range tables {
		readyForConversion := true
		for _, col := range table.Columns() {
			if !(len(col.Indexes) > 0 || col.IsPrimaryKey) {
				continue
			}
			if !(col.SQLType.Name == "VARCHAR" && col.Length > maxvc) {
				continue
			}
			log.Info("reducing column %s.%s from %d to %d bytes", table.Name, col.Name, col.Length, maxvc)
			sqlstmt := fmt.Sprintf("alter table `%s` change column `%s` `%s` varchar(%d)", table.Name, col.Name, col.Name, maxvc)
			if _, err := x.Exec(sqlstmt); err != nil {
				if e, ok := err.(*mysql.MySQLError); ok {
					if e.Number == 1265 || e.Number == 1406 {
						log.Warn("failed. Please cut all data of this column down to a maximum of %d bytes", maxvc)
					} else {
						log.Warn("failed with %v", err)
					}
					readyForConversion = false
					migrationSuccess = false
					continue
				}
				return fmt.Errorf("something went horribly wrong: %v", err)
			}
		}
		if readyForConversion {
			log.Info("%s: converting table to utf8mb4", table.Name)
			if _, err := x.Exec("alter table `" + table.Name + "` convert to character set utf8mb4"); err != nil {
				log.Warn("conversion of %s failed: %v", table.Name, err)
				migrationSuccess = false
			}
		}
	}
	if !migrationSuccess {
		return fmt.Errorf("conversion of some of the tables failed. Please check the logs and re-run gitea")
	}
	return nil
}
