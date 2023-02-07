// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
	"xorm.io/xorm/convert"
	"xorm.io/xorm/dialects"
	"xorm.io/xorm/schemas"
)

// BuildCaseInsensitiveLike returns a condition to check if the given value is like the given key case-insensitively.
// Handles especially SQLite correctly as UPPER there only transforms ASCII letters.
func BuildCaseInsensitiveLike(key, value string) builder.Cond {
	if setting.Database.UseSQLite3 {
		return builder.Like{"UPPER(" + key + ")", util.ToUpperASCII(value)}
	}
	return builder.Like{"UPPER(" + key + ")", strings.ToUpper(value)}
}

func InsertOnConflictDoNothing(ctx context.Context, bean interface{}) (int64, error) {
	e := GetEngine(ctx)

	tableName := x.TableName(bean, true)
	table, err := x.TableInfo(bean)
	if err != nil {
		return 0, err
	}

	autoIncrCol := table.AutoIncrColumn()

	cols := table.Columns()
	colNames := make([]string, 0, len(cols))
	args := make([]any, 1, len(cols))
	emptyColNames := make([]string, 0, len(cols))
	emptyArgs := make([]any, 0, len(cols))

	val := reflect.ValueOf(bean)
	elem := val.Elem()
	for _, col := range cols {
		if fieldIdx := col.FieldIndex; fieldIdx != nil {
			fieldVal := elem.FieldByIndex(fieldIdx)
			if col.IsCreated || col.IsUpdated {
				t := time.Now()
				result, err := dialects.FormatColumnTime(x.Dialect(), x.DatabaseTZ, col, t)
				if err != nil {
					return 0, err
				}

				switch fieldVal.Type().Kind() {
				case reflect.Struct:
					fieldVal.Set(reflect.ValueOf(t).Convert(fieldVal.Type()))
				case reflect.Int, reflect.Int64, reflect.Int32:
					fieldVal.SetInt(t.Unix())
				case reflect.Uint, reflect.Uint64, reflect.Uint32:
					fieldVal.SetUint(uint64(t.Unix()))
				}

				colNames = append(colNames, col.Name)
				args = append(args, result)
				continue
			}

			if fieldVal.IsZero() {
				emptyColNames = append(emptyColNames, col.Name)
				switch fieldVal.Type().Kind() {
				case reflect.Int, reflect.Int8, reflect.Uint8, reflect.Int16, reflect.Uint16, reflect.Int32, reflect.Uint32, reflect.Int64, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
					emptyArgs = append(emptyArgs, 0)
				case reflect.String:
					emptyArgs = append(emptyArgs, "")
				case reflect.Bool:
					emptyArgs = append(emptyArgs, false)
				default:
					emptyArgs = append(emptyArgs, nil)
				}
				continue
			}
			colNames = append(colNames, col.Name)
			args = append(args, fieldVal.Interface())
		}
	}

	if len(colNames) == 0 {
		return 0, fmt.Errorf("empty bean")
	}

	uniqueCols := make([]string, 0, len(cols))
	uniqueArgs := make([]interface{}, 1, len(cols))
	for _, index := range table.Indexes {
		if index.Type != schemas.UniqueType {
			continue
		}
	indexCol:
		for _, iCol := range index.Cols {
			for _, uCol := range uniqueCols {
				if uCol == iCol {
					continue indexCol
				}
			}
			for i, col := range colNames {
				if col == iCol {
					uniqueCols = append(uniqueCols, col)
					uniqueArgs = append(uniqueArgs, args[i+1])
					continue indexCol
				}
			}
			for i, col := range emptyColNames {
				if col == iCol {
					colNames = append(colNames, col)
					args = append(args, emptyArgs[i])
					uniqueCols = append(uniqueCols, col)
					uniqueArgs = append(uniqueArgs, emptyArgs[i])
					continue indexCol
				}
			}
		}
	}

	if len(uniqueCols) == 0 {
		return 0, fmt.Errorf("empty bean")
	}

	sb := &strings.Builder{}
	switch {
	case setting.Database.UseSQLite3 || setting.Database.UsePostgreSQL || setting.Database.UseMySQL:
		_, _ = sb.WriteString("INSERT ")
		if setting.Database.UseMySQL && autoIncrCol == nil {
			_, _ = sb.WriteString("IGNORE ")
		}
		_, _ = sb.WriteString("INTO ")
		_, _ = sb.WriteString(x.Dialect().Quoter().Quote(tableName))
		_, _ = sb.WriteString(" (")
		_, _ = sb.WriteString(colNames[0])
		for _, colName := range colNames[1:] {
			_, _ = sb.WriteString(",")
			_, _ = sb.WriteString(colName)
		}
		_, _ = sb.WriteString(") VALUES (")
		_, _ = sb.WriteString("?")
		for range colNames[1:] {
			_, _ = sb.WriteString(",?")
		}
		switch {
		case setting.Database.UsePostgreSQL:
			if autoIncrCol != nil {
				_, _ = fmt.Fprintf(sb, ") RETURNING %s ON CONFLICT DO NOTHING", x.Dialect().Quoter().Quote(autoIncrCol.Name))
			} else {
				_, _ = sb.WriteString(") ON CONFLICT DO NOTHING")
			}
		case setting.Database.UseSQLite3:
			_, _ = sb.WriteString(") ON CONFLICT DO NOTHING")
		case setting.Database.UseMySQL:
			if autoIncrCol != nil {
				_, _ = sb.WriteString(") ON DUPLICATE KEY UPDATE ")
				_, _ = sb.WriteString(autoIncrCol.Name)
				_, _ = sb.WriteString(" = ")
				_, _ = sb.WriteString(autoIncrCol.Name)
			}
		}
	case setting.Database.UseMSSQL:
		_, _ = sb.WriteString("MERGE ")
		_, _ = sb.WriteString(x.Dialect().Quoter().Quote(tableName))
		_, _ = sb.WriteString(" WITH (HOLDLOCK) AS target USING (SELECT ")

		_, _ = sb.WriteString("? AS ")
		_, _ = sb.WriteString(uniqueCols[0])
		for _, uniqueCol := range uniqueCols[1:] {
			_, _ = sb.WriteString(", ? AS ")
			_, _ = sb.WriteString(uniqueCol)
		}
		_, _ = sb.WriteString(") AS src ON src.")
		_, _ = sb.WriteString(uniqueCols[0])
		_, _ = sb.WriteString("= target.")
		_, _ = sb.WriteString(uniqueCols[0])
		for _, uniqueCol := range uniqueCols[1:] {
			_, _ = sb.WriteString(" AND src.")
			_, _ = sb.WriteString(uniqueCol)
			_, _ = sb.WriteString("= target.")
			_, _ = sb.WriteString(uniqueCols[0])
		}
		_, _ = sb.WriteString(" WHEN NOT MATCHED THEN INSERT (")
		_, _ = sb.WriteString(colNames[0])
		for _, colName := range colNames[1:] {
			_, _ = sb.WriteString(",")
			_, _ = sb.WriteString(colName)
		}
		_, _ = sb.WriteString(") VALUES (")
		_, _ = sb.WriteString("?")
		for range colNames[1:] {
			_, _ = sb.WriteString(",?")
		}
		_, _ = sb.WriteString(");")
		args = append(uniqueArgs, args[1:]...)
	default:
		return 0, fmt.Errorf("database type not supported")
	}
	args[0] = sb.String()

	if autoIncrCol != nil {
		switch {
		case setting.Database.UsePostgreSQL:
			res, err := e.Query(args...)
			if err != nil {
				return 0, err
			}
			if len(res) == 0 {
				return 0, nil
			}

			aiValue, err := table.AutoIncrColumn().ValueOf(bean)
			if err != nil {
				log.Error("unable to get value for autoincrcol of %#v %v", bean, err)
			}

			if aiValue == nil || !aiValue.IsValid() || !aiValue.CanSet() {
				return int64(len(res)), nil
			}

			id := res[0][autoIncrCol.Name]
			return int64(len(res)), convert.AssignValue(*aiValue, id)
		}
	}
	res, err := e.Exec(args...)
	if err != nil {
		return 0, err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return n, err
	}

	if n != 0 && autoIncrCol != nil {
		id, err := res.LastInsertId()
		if err != nil {
			return n, err
		}
		elem.FieldByName(autoIncrCol.FieldName).SetInt(id)
	}

	return res.RowsAffected()
}
