// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13 //nolint

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func SetDefaultPasswordToArgon2(x *xorm.Engine) error {
	switch {
	case setting.Database.Type.IsMySQL():
		_, err := x.Exec("ALTER TABLE `user` ALTER passwd_hash_algo SET DEFAULT 'argon2';")
		return err
	case setting.Database.Type.IsPostgreSQL():
		_, err := x.Exec("ALTER TABLE `user` ALTER COLUMN passwd_hash_algo SET DEFAULT 'argon2';")
		return err
	case setting.Database.Type.IsMSSQL():
		// need to find the constraint and drop it, then recreate it.
		sess := x.NewSession()
		defer sess.Close()
		if err := sess.Begin(); err != nil {
			return err
		}
		res, err := sess.QueryString("SELECT [name] FROM sys.default_constraints WHERE parent_object_id=OBJECT_ID(?) AND COL_NAME(parent_object_id, parent_column_id)=?;", "user", "passwd_hash_algo")
		if err != nil {
			return err
		}
		if len(res) > 0 {
			constraintName := res[0]["name"]
			log.Error("Results of select constraint: %s", constraintName)
			_, err := sess.Exec("ALTER TABLE [user] DROP CONSTRAINT " + constraintName)
			if err != nil {
				return err
			}
			_, err = sess.Exec("ALTER TABLE [user] ADD CONSTRAINT " + constraintName + " DEFAULT 'argon2' FOR passwd_hash_algo")
			if err != nil {
				return err
			}
		} else {
			_, err := sess.Exec("ALTER TABLE [user] ADD DEFAULT('argon2') FOR passwd_hash_algo")
			if err != nil {
				return err
			}
		}
		return sess.Commit()

	case setting.Database.Type.IsSQLite3():
		// drop through
	default:
		log.Fatal("Unrecognized DB")
	}

	tables, err := x.DBMetas()
	if err != nil {
		return err
	}

	// Now for SQLite we have to recreate the table
	var table *schemas.Table
	tableName := "user"

	for _, table = range tables {
		if table.Name == tableName {
			break
		}
	}
	if table == nil || table.Name != tableName {
		type User struct {
			PasswdHashAlgo string `xorm:"NOT NULL DEFAULT 'argon2'"`
		}
		return x.Sync(new(User))
	}
	column := table.GetColumn("passwd_hash_algo")
	if column == nil {
		type User struct {
			PasswdHashAlgo string `xorm:"NOT NULL DEFAULT 'argon2'"`
		}
		return x.Sync(new(User))
	}

	tempTableName := "tmp_recreate__user"
	column.Default = "'argon2'"

	createTableSQL, _, err := x.Dialect().CreateTableSQL(context.Background(), x.DB(), table, tempTableName)
	if err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if _, err := sess.Exec(createTableSQL); err != nil {
		log.Error("Unable to create table %s. Error: %v\n", tempTableName, err, createTableSQL)
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

	return sess.Commit()
}
