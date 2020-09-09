// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func setDefaultPasswordToArgon2(x *xorm.Engine) error {
	tables, err := x.DBMetas()
	if err != nil {
		return err
	}

	var table *schemas.Table
	tableName := "user"
	tempTableName := "tmp_recreate__user"

	for _, table = range tables {
		if table.Name == tableName {
			break
		}
	}
	if table == nil || table.Name != tableName {
		type User struct {
			PasswdHashAlgo string `xorm:"NOT NULL DEFAULT 'argon2'"`
		}
		return x.Sync2(new(User))
	}
	column := table.GetColumn("passwd_hash_algo")
	if column == nil {
		type User struct {
			PasswdHashAlgo string `xorm:"NOT NULL DEFAULT 'argon2'"`
		}
		return x.Sync2(new(User))
	}
	column.Default = "argon2"

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	table.StoreEngine = "InnoDB"
	if _, err := sess.Exec(x.Dialect().CreateTableSQL(table, tempTableName)); err != nil {
		log.Error("Unable to create table %s. Error: %v", tempTableName, err)
		return err
	}
	for _, index := range table.Indexes {
		if _, err := sess.Exec(x.Dialect().CreateIndexSQL(tempTableName, index)); err != nil {
			log.Error("Unable to create indexes on temporary table %s. Error: %v", tempTableName, err)
			return err
		}
	}

	newTableColumns := table.Columns()
	if len(newTableColumns) == 0 {
		return fmt.Errorf("no columns in new table")
	}
	hasID := false
	for _, column := range newTableColumns {
		hasID = hasID || (column.IsPrimaryKey && column.IsAutoIncrement)
	}

	if hasID && setting.Database.UseMSSQL {
		if _, err := sess.Exec(fmt.Sprintf("SET IDENTITY_INSERT `%s` ON", tempTableName)); err != nil {
			log.Error("Unable to set identity insert for table %s. Error: %v", tempTableName, err)
			return err
		}
	}

	sqlStringBuilder := &strings.Builder{}
	_, _ = sqlStringBuilder.WriteString("INSERT INTO `")
	_, _ = sqlStringBuilder.WriteString(tempTableName)
	_, _ = sqlStringBuilder.WriteString("` (`")
	_, _ = sqlStringBuilder.WriteString(newTableColumns[0].Name)
	_, _ = sqlStringBuilder.WriteString("`")
	for _, column := range newTableColumns[1:] {
		_, _ = sqlStringBuilder.WriteString(", `")
		_, _ = sqlStringBuilder.WriteString(column.Name)
		_, _ = sqlStringBuilder.WriteString("`")
	}
	_, _ = sqlStringBuilder.WriteString(")")
	_, _ = sqlStringBuilder.WriteString(" SELECT ")
	if newTableColumns[0].Default != "" {
		_, _ = sqlStringBuilder.WriteString("COALESCE(`")
		_, _ = sqlStringBuilder.WriteString(newTableColumns[0].Name)
		_, _ = sqlStringBuilder.WriteString("`, ")
		_, _ = sqlStringBuilder.WriteString(newTableColumns[0].Default)
		_, _ = sqlStringBuilder.WriteString(")")
	} else {
		_, _ = sqlStringBuilder.WriteString("`")
		_, _ = sqlStringBuilder.WriteString(newTableColumns[0].Name)
		_, _ = sqlStringBuilder.WriteString("`")
	}

	for _, column := range newTableColumns[1:] {
		if column.Default != "" {
			_, _ = sqlStringBuilder.WriteString(", COALESCE(`")
			_, _ = sqlStringBuilder.WriteString(column.Name)
			_, _ = sqlStringBuilder.WriteString("`, ")
			_, _ = sqlStringBuilder.WriteString(column.Default)
			_, _ = sqlStringBuilder.WriteString(")")
		} else {
			_, _ = sqlStringBuilder.WriteString(", `")
			_, _ = sqlStringBuilder.WriteString(column.Name)
			_, _ = sqlStringBuilder.WriteString("`")
		}
	}
	_, _ = sqlStringBuilder.WriteString(" FROM `")
	_, _ = sqlStringBuilder.WriteString(tableName)
	_, _ = sqlStringBuilder.WriteString("`")

	if _, err := sess.Exec(sqlStringBuilder.String()); err != nil {
		log.Error("Unable to set copy data in to temp table %s. Error: %v", tempTableName, err)
		return err
	}

	if hasID && setting.Database.UseMSSQL {
		if _, err := sess.Exec(fmt.Sprintf("SET IDENTITY_INSERT `%s` OFF", tempTableName)); err != nil {
			log.Error("Unable to switch off identity insert for table %s. Error: %v", tempTableName, err)
			return err
		}
	}

	switch {
	case setting.Database.UseSQLite3:
		// SQLite will drop all the constraints on the old table
		if _, err := sess.Exec(fmt.Sprintf("DROP TABLE `%s`", tableName)); err != nil {
			log.Error("Unable to drop old table %s. Error: %v", tableName, err)
			return err
		}

		for _, index := range table.Indexes {
			if _, err := sess.Exec(x.Dialect().DropIndexSQL(tempTableName, index)); err != nil {
				log.Error("Unable to drop indexes on temporary table %s. Error: %v", tempTableName, err)
				return err
			}
		}

		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", tempTableName, tableName)); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}

		for _, index := range table.Indexes {
			if _, err := sess.Exec(x.Dialect().CreateIndexSQL(tableName, index)); err != nil {
				log.Error("Unable to recreate indexes on table %s. Error: %v", tableName, err)
				return err
			}
		}

	case setting.Database.UseMySQL:
		// MySQL will drop all the constraints on the old table
		if _, err := sess.Exec(fmt.Sprintf("DROP TABLE `%s`", tableName)); err != nil {
			log.Error("Unable to drop old table %s. Error: %v", tableName, err)
			return err
		}

		// SQLite and MySQL will move all the constraints from the temporary table to the new table
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", tempTableName, tableName)); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}
	case setting.Database.UsePostgreSQL:
		// CASCADE causes postgres to drop all the constraints on the old table
		if _, err := sess.Exec(fmt.Sprintf("DROP TABLE `%s` CASCADE", tableName)); err != nil {
			log.Error("Unable to drop old table %s. Error: %v", tableName, err)
			return err
		}

		// CASCADE causes postgres to move all the constraints from the temporary table to the new table
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", tempTableName, tableName)); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}

		var indices []string
		schema := sess.Engine().Dialect().URI().Schema
		sess.Engine().SetSchema("")
		if err := sess.Table("pg_indexes").Cols("indexname").Where("tablename = ? ", tableName).Find(&indices); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}
		sess.Engine().SetSchema(schema)

		for _, index := range indices {
			newIndexName := strings.Replace(index, "tmp_recreate__", "", 1)
			if _, err := sess.Exec(fmt.Sprintf("ALTER INDEX `%s` RENAME TO `%s`", index, newIndexName)); err != nil {
				log.Error("Unable to rename %s to %s. Error: %v", index, newIndexName, err)
				return err
			}
		}

	case setting.Database.UseMSSQL:
		// MSSQL will drop all the constraints on the old table
		if _, err := sess.Exec(fmt.Sprintf("DROP TABLE `%s`", tableName)); err != nil {
			log.Error("Unable to drop old table %s. Error: %v", tableName, err)
			return err
		}

		// MSSQL sp_rename will move all the constraints from the temporary table to the new table
		if _, err := sess.Exec(fmt.Sprintf("sp_rename `%s`,`%s`", tempTableName, tableName)); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}

	default:
		log.Fatal("Unrecognized DB")
	}

	return sess.Commit()
}
