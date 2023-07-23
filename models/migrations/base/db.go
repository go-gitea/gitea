// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path"
	"reflect"
	"regexp"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// RecreateTables will recreate the tables for the provided beans using the newly provided bean definition and move all data to that new table
// WARNING: YOU MUST PROVIDE THE FULL BEAN DEFINITION
func RecreateTables(beans ...any) func(*xorm.Engine) error {
	return func(x *xorm.Engine) error {
		sess := x.NewSession()
		defer sess.Close()
		if err := sess.Begin(); err != nil {
			return err
		}
		sess = sess.StoreEngine("InnoDB")
		for _, bean := range beans {
			log.Info("Recreating Table: %s for Bean: %s", x.TableName(bean), reflect.Indirect(reflect.ValueOf(bean)).Type().Name())
			if err := RecreateTable(sess, bean); err != nil {
				return err
			}
		}
		return sess.Commit()
	}
}

// RecreateTable will recreate the table using the newly provided bean definition and move all data to that new table
// WARNING: YOU MUST PROVIDE THE FULL BEAN DEFINITION
// WARNING: YOU MUST COMMIT THE SESSION AT THE END
func RecreateTable(sess *xorm.Session, bean any) error {
	// TODO: This will not work if there are foreign keys

	tableName := sess.Engine().TableName(bean)
	tempTableName := fmt.Sprintf("tmp_recreate__%s", tableName)

	// We need to move the old table away and create a new one with the correct columns
	// We will need to do this in stages to prevent data loss
	//
	// First create the temporary table
	if err := sess.Table(tempTableName).CreateTable(bean); err != nil {
		log.Error("Unable to create table %s. Error: %v", tempTableName, err)
		return err
	}

	if err := sess.Table(tempTableName).CreateUniques(bean); err != nil {
		log.Error("Unable to create uniques for table %s. Error: %v", tempTableName, err)
		return err
	}

	if err := sess.Table(tempTableName).CreateIndexes(bean); err != nil {
		log.Error("Unable to create indexes for table %s. Error: %v", tempTableName, err)
		return err
	}

	// Work out the column names from the bean - these are the columns to select from the old table and install into the new table
	table, err := sess.Engine().TableInfo(bean)
	if err != nil {
		log.Error("Unable to get table info. Error: %v", err)

		return err
	}
	newTableColumns := table.Columns()
	if len(newTableColumns) == 0 {
		return fmt.Errorf("no columns in new table")
	}
	hasID := false
	for _, column := range newTableColumns {
		hasID = hasID || (column.IsPrimaryKey && column.IsAutoIncrement)
	}

	if hasID && setting.Database.Type.IsMSSQL() {
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

	if hasID && setting.Database.Type.IsMSSQL() {
		if _, err := sess.Exec(fmt.Sprintf("SET IDENTITY_INSERT `%s` OFF", tempTableName)); err != nil {
			log.Error("Unable to switch off identity insert for table %s. Error: %v", tempTableName, err)
			return err
		}
	}

	switch {
	case setting.Database.Type.IsSQLite3():
		// SQLite will drop all the constraints on the old table
		if _, err := sess.Exec(fmt.Sprintf("DROP TABLE `%s`", tableName)); err != nil {
			log.Error("Unable to drop old table %s. Error: %v", tableName, err)
			return err
		}

		if err := sess.Table(tempTableName).DropIndexes(bean); err != nil {
			log.Error("Unable to drop indexes on temporary table %s. Error: %v", tempTableName, err)
			return err
		}

		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", tempTableName, tableName)); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}

		if err := sess.Table(tableName).CreateIndexes(bean); err != nil {
			log.Error("Unable to recreate indexes on table %s. Error: %v", tableName, err)
			return err
		}

		if err := sess.Table(tableName).CreateUniques(bean); err != nil {
			log.Error("Unable to recreate uniques on table %s. Error: %v", tableName, err)
			return err
		}

	case setting.Database.Type.IsMySQL():
		// MySQL will drop all the constraints on the old table
		if _, err := sess.Exec(fmt.Sprintf("DROP TABLE `%s`", tableName)); err != nil {
			log.Error("Unable to drop old table %s. Error: %v", tableName, err)
			return err
		}

		if err := sess.Table(tempTableName).DropIndexes(bean); err != nil {
			log.Error("Unable to drop indexes on temporary table %s. Error: %v", tempTableName, err)
			return err
		}

		// SQLite and MySQL will move all the constraints from the temporary table to the new table
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", tempTableName, tableName)); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}

		if err := sess.Table(tableName).CreateIndexes(bean); err != nil {
			log.Error("Unable to recreate indexes on table %s. Error: %v", tableName, err)
			return err
		}

		if err := sess.Table(tableName).CreateUniques(bean); err != nil {
			log.Error("Unable to recreate uniques on table %s. Error: %v", tableName, err)
			return err
		}
	case setting.Database.Type.IsPostgreSQL():
		var originalSequences []string
		type sequenceData struct {
			LastValue int  `xorm:"'last_value'"`
			IsCalled  bool `xorm:"'is_called'"`
		}
		sequenceMap := map[string]sequenceData{}

		schema := sess.Engine().Dialect().URI().Schema
		sess.Engine().SetSchema("")
		if err := sess.Table("information_schema.sequences").Cols("sequence_name").Where("sequence_name LIKE ? || '_%' AND sequence_catalog = ?", tableName, setting.Database.Name).Find(&originalSequences); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}
		sess.Engine().SetSchema(schema)

		for _, sequence := range originalSequences {
			sequenceData := sequenceData{}
			if _, err := sess.Table(sequence).Cols("last_value", "is_called").Get(&sequenceData); err != nil {
				log.Error("Unable to get last_value and is_called from %s. Error: %v", sequence, err)
				return err
			}
			sequenceMap[sequence] = sequenceData

		}

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

		var sequences []string
		sess.Engine().SetSchema("")
		if err := sess.Table("information_schema.sequences").Cols("sequence_name").Where("sequence_name LIKE 'tmp_recreate__' || ? || '_%' AND sequence_catalog = ?", tableName, setting.Database.Name).Find(&sequences); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}
		sess.Engine().SetSchema(schema)

		for _, sequence := range sequences {
			newSequenceName := strings.Replace(sequence, "tmp_recreate__", "", 1)
			if _, err := sess.Exec(fmt.Sprintf("ALTER SEQUENCE `%s` RENAME TO `%s`", sequence, newSequenceName)); err != nil {
				log.Error("Unable to rename %s sequence to %s. Error: %v", sequence, newSequenceName, err)
				return err
			}
			val, ok := sequenceMap[newSequenceName]
			if newSequenceName == tableName+"_id_seq" {
				if ok && val.LastValue != 0 {
					if _, err := sess.Exec(fmt.Sprintf("SELECT setval('%s', %d, %t)", newSequenceName, val.LastValue, val.IsCalled)); err != nil {
						log.Error("Unable to reset %s to %d. Error: %v", newSequenceName, val, err)
						return err
					}
				} else {
					// We're going to try to guess this
					if _, err := sess.Exec(fmt.Sprintf("SELECT setval('%s', COALESCE((SELECT MAX(id)+1 FROM `%s`), 1), false)", newSequenceName, tableName)); err != nil {
						log.Error("Unable to reset %s. Error: %v", newSequenceName, err)
						return err
					}
				}
			} else if ok {
				if _, err := sess.Exec(fmt.Sprintf("SELECT setval('%s', %d, %t)", newSequenceName, val.LastValue, val.IsCalled)); err != nil {
					log.Error("Unable to reset %s to %d. Error: %v", newSequenceName, val, err)
					return err
				}
			}

		}

	case setting.Database.Type.IsMSSQL():
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
	return nil
}

// WARNING: YOU MUST COMMIT THE SESSION AT THE END
func DropTableColumns(sess *xorm.Session, tableName string, columnNames ...string) (err error) {
	if tableName == "" || len(columnNames) == 0 {
		return nil
	}
	// TODO: This will not work if there are foreign keys

	switch {
	case setting.Database.Type.IsSQLite3():
		// First drop the indexes on the columns
		res, errIndex := sess.Query(fmt.Sprintf("PRAGMA index_list(`%s`)", tableName))
		if errIndex != nil {
			return errIndex
		}
		for _, row := range res {
			indexName := row["name"]
			indexRes, err := sess.Query(fmt.Sprintf("PRAGMA index_info(`%s`)", indexName))
			if err != nil {
				return err
			}
			if len(indexRes) != 1 {
				continue
			}
			indexColumn := string(indexRes[0]["name"])
			for _, name := range columnNames {
				if name == indexColumn {
					_, err := sess.Exec(fmt.Sprintf("DROP INDEX `%s`", indexName))
					if err != nil {
						return err
					}
				}
			}
		}

		// Here we need to get the columns from the original table
		sql := fmt.Sprintf("SELECT sql FROM sqlite_master WHERE tbl_name='%s' and type='table'", tableName)
		res, err := sess.Query(sql)
		if err != nil {
			return err
		}
		tableSQL := string(res[0]["sql"])

		// Get the string offset for column definitions: `CREATE TABLE ( column-definitions... )`
		columnDefinitionsIndex := strings.Index(tableSQL, "(")
		if columnDefinitionsIndex < 0 {
			return errors.New("couldn't find column definitions")
		}

		// Separate out the column definitions
		tableSQL = tableSQL[columnDefinitionsIndex:]

		// Remove the required columnNames
		for _, name := range columnNames {
			tableSQL = regexp.MustCompile(regexp.QuoteMeta("`"+name+"`")+"[^`,)]*?[,)]").ReplaceAllString(tableSQL, "")
		}

		// Ensure the query is ended properly
		tableSQL = strings.TrimSpace(tableSQL)
		if tableSQL[len(tableSQL)-1] != ')' {
			if tableSQL[len(tableSQL)-1] == ',' {
				tableSQL = tableSQL[:len(tableSQL)-1]
			}
			tableSQL += ")"
		}

		// Find all the columns in the table
		columns := regexp.MustCompile("`([^`]*)`").FindAllString(tableSQL, -1)

		tableSQL = fmt.Sprintf("CREATE TABLE `new_%s_new` ", tableName) + tableSQL
		if _, err := sess.Exec(tableSQL); err != nil {
			return err
		}

		// Now restore the data
		columnsSeparated := strings.Join(columns, ",")
		insertSQL := fmt.Sprintf("INSERT INTO `new_%s_new` (%s) SELECT %s FROM %s", tableName, columnsSeparated, columnsSeparated, tableName)
		if _, err := sess.Exec(insertSQL); err != nil {
			return err
		}

		// Now drop the old table
		if _, err := sess.Exec(fmt.Sprintf("DROP TABLE `%s`", tableName)); err != nil {
			return err
		}

		// Rename the table
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `new_%s_new` RENAME TO `%s`", tableName, tableName)); err != nil {
			return err
		}

	case setting.Database.Type.IsPostgreSQL():
		cols := ""
		for _, col := range columnNames {
			if cols != "" {
				cols += ", "
			}
			cols += "DROP COLUMN `" + col + "` CASCADE"
		}
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` %s", tableName, cols)); err != nil {
			return fmt.Errorf("Drop table `%s` columns %v: %v", tableName, columnNames, err)
		}
	case setting.Database.Type.IsMySQL():
		// Drop indexes on columns first
		sql := fmt.Sprintf("SHOW INDEX FROM %s WHERE column_name IN ('%s')", tableName, strings.Join(columnNames, "','"))
		res, err := sess.Query(sql)
		if err != nil {
			return err
		}
		for _, index := range res {
			indexName := index["column_name"]
			if len(indexName) > 0 {
				_, err := sess.Exec(fmt.Sprintf("DROP INDEX `%s` ON `%s`", indexName, tableName))
				if err != nil {
					return err
				}
			}
		}

		// Now drop the columns
		cols := ""
		for _, col := range columnNames {
			if cols != "" {
				cols += ", "
			}
			cols += "DROP COLUMN `" + col + "`"
		}
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` %s", tableName, cols)); err != nil {
			return fmt.Errorf("Drop table `%s` columns %v: %v", tableName, columnNames, err)
		}
	case setting.Database.Type.IsMSSQL():
		cols := ""
		for _, col := range columnNames {
			if cols != "" {
				cols += ", "
			}
			cols += "`" + strings.ToLower(col) + "`"
		}
		sql := fmt.Sprintf("SELECT Name FROM sys.default_constraints WHERE parent_object_id = OBJECT_ID('%[1]s') AND parent_column_id IN (SELECT column_id FROM sys.columns WHERE LOWER(name) IN (%[2]s) AND object_id = OBJECT_ID('%[1]s'))",
			tableName, strings.ReplaceAll(cols, "`", "'"))
		constraints := make([]string, 0)
		if err := sess.SQL(sql).Find(&constraints); err != nil {
			return fmt.Errorf("Find constraints: %v", err)
		}
		for _, constraint := range constraints {
			if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` DROP CONSTRAINT `%s`", tableName, constraint)); err != nil {
				return fmt.Errorf("Drop table `%s` default constraint `%s`: %v", tableName, constraint, err)
			}
		}
		sql = fmt.Sprintf("SELECT DISTINCT Name FROM sys.indexes INNER JOIN sys.index_columns ON indexes.index_id = index_columns.index_id AND indexes.object_id = index_columns.object_id WHERE indexes.object_id = OBJECT_ID('%[1]s') AND index_columns.column_id IN (SELECT column_id FROM sys.columns WHERE LOWER(name) IN (%[2]s) AND object_id = OBJECT_ID('%[1]s'))",
			tableName, strings.ReplaceAll(cols, "`", "'"))
		constraints = make([]string, 0)
		if err := sess.SQL(sql).Find(&constraints); err != nil {
			return fmt.Errorf("Find constraints: %v", err)
		}
		for _, constraint := range constraints {
			if _, err := sess.Exec(fmt.Sprintf("DROP INDEX `%[2]s` ON `%[1]s`", tableName, constraint)); err != nil {
				return fmt.Errorf("Drop index `%[2]s` on `%[1]s`: %v", tableName, constraint, err)
			}
		}

		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN %s", tableName, cols)); err != nil {
			return fmt.Errorf("Drop table `%s` columns %v: %v", tableName, columnNames, err)
		}
	default:
		log.Fatal("Unrecognized DB")
	}

	return nil
}

// ModifyColumn will modify column's type or other property. SQLITE is not supported
func ModifyColumn(x *xorm.Engine, tableName string, col *schemas.Column) error {
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

func removeAllWithRetry(dir string) error {
	var err error
	for i := 0; i < 20; i++ {
		err = os.RemoveAll(dir)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

func newXORMEngine() (*xorm.Engine, error) {
	if err := db.InitEngine(context.Background()); err != nil {
		return nil, err
	}
	x := unittest.GetXORMEngine()
	return x, nil
}

func deleteDB() error {
	switch {
	case setting.Database.Type.IsSQLite3():
		if err := util.Remove(setting.Database.Path); err != nil {
			return err
		}
		return os.MkdirAll(path.Dir(setting.Database.Path), os.ModePerm)

	case setting.Database.Type.IsMySQL():
		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host))
		if err != nil {
			return err
		}
		defer db.Close()

		if _, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", setting.Database.Name)); err != nil {
			return err
		}

		if _, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", setting.Database.Name)); err != nil {
			return err
		}
		return nil
	case setting.Database.Type.IsPostgreSQL():
		db, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/?sslmode=%s",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.SSLMode))
		if err != nil {
			return err
		}
		defer db.Close()

		if _, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", setting.Database.Name)); err != nil {
			return err
		}

		if _, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", setting.Database.Name)); err != nil {
			return err
		}
		db.Close()

		// Check if we need to setup a specific schema
		if len(setting.Database.Schema) != 0 {
			db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
				setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.Name, setting.Database.SSLMode))
			if err != nil {
				return err
			}
			defer db.Close()

			schrows, err := db.Query(fmt.Sprintf("SELECT 1 FROM information_schema.schemata WHERE schema_name = '%s'", setting.Database.Schema))
			if err != nil {
				return err
			}
			defer schrows.Close()

			if !schrows.Next() {
				// Create and setup a DB schema
				_, err = db.Exec(fmt.Sprintf("CREATE SCHEMA %s", setting.Database.Schema))
				if err != nil {
					return err
				}
			}

			// Make the user's default search path the created schema; this will affect new connections
			_, err = db.Exec(fmt.Sprintf(`ALTER USER "%s" SET search_path = %s`, setting.Database.User, setting.Database.Schema))
			if err != nil {
				return err
			}
			return nil
		}
	case setting.Database.Type.IsMSSQL():
		host, port := setting.ParseMSSQLHostPort(setting.Database.Host)
		db, err := sql.Open("mssql", fmt.Sprintf("server=%s; port=%s; database=%s; user id=%s; password=%s;",
			host, port, "master", setting.Database.User, setting.Database.Passwd))
		if err != nil {
			return err
		}
		defer db.Close()

		if _, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS [%s]", setting.Database.Name)); err != nil {
			return err
		}
		if _, err = db.Exec(fmt.Sprintf("CREATE DATABASE [%s]", setting.Database.Name)); err != nil {
			return err
		}
	}

	return nil
}
