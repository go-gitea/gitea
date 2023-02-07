// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"fmt"
	"io"
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

// InsertOnConflictDoNothing will attempt to insert the provided bean but if there is a conflict it will not error out
// This function will update the ID of the provided bean if there is an insertion
// This does not do all of the conversions that xorm would do automatically but it does quite a number of them
// once xorm has a working InsertOnConflictDoNothing this function could be removed.
func InsertOnConflictDoNothing(ctx context.Context, bean interface{}) (bool, error) {
	e := GetEngine(ctx)

	tableName := x.TableName(bean, true)
	table, err := x.TableInfo(bean)
	if err != nil {
		return false, err
	}

	autoIncrCol := table.AutoIncrColumn()

	cols := table.Columns()

	colNames, args, emptyColNames, emptyArgs, err := getColNamesAndArgsFromBean(bean, cols)
	if err != nil {
		return false, err
	}

	if len(colNames) == 0 {
		return false, fmt.Errorf("provided bean to insert has all empty values")
	}

	// MSSQL needs to separately pass in the columns with the unique constraint and we need to
	// include empty columns which are in the constraint in the insert for other dbs
	uniqueCols, uniqueArgs, colNames, args := addInUniqueCols(colNames, args, emptyColNames, emptyArgs, table)
	if len(uniqueCols) == 0 {
		return false, fmt.Errorf("provided bean has no unique constraints")
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
			_, _ = sb.WriteString(") ON CONFLICT DO NOTHING")
			if autoIncrCol != nil {
				_, _ = fmt.Fprintf(sb, " RETURNING %s", autoIncrCol.Name)
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
		generateInsertNoConflictSQLForMSSQL(sb, tableName, colNames, args, uniqueCols, autoIncrCol)
		args = append(uniqueArgs, args[1:]...)
	default:
		return false, fmt.Errorf("database type not supported")
	}
	args[0] = sb.String()

	if autoIncrCol != nil && (setting.Database.UsePostgreSQL || setting.Database.UseMSSQL) {
		// Postgres and MSSQL do not use the LastInsertID mechanism
		// Therefore use query rather than exec and read the last provided ID back in

		res, err := e.Query(args...)
		if err != nil {
			return false, fmt.Errorf("error in query: %s, %w", args[0], err)
		}
		if len(res) == 0 {
			// this implies there was a conflict
			return false, nil
		}

		aiValue, err := table.AutoIncrColumn().ValueOf(bean)
		if err != nil {
			log.Error("unable to get value for autoincrcol of %#v %v", bean, err)
		}

		if aiValue == nil || !aiValue.IsValid() || !aiValue.CanSet() {
			return true, nil
		}

		id := res[0][autoIncrCol.Name]
		err = convert.AssignValue(*aiValue, id)
		if err != nil {
			return true, fmt.Errorf("error in assignvalue %v %v %w", id, res, err)
		}
		return true, nil
	}

	res, err := e.Exec(args...)
	if err != nil {
		return false, err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return n != 0, err
	}

	if n != 0 && autoIncrCol != nil {
		id, err := res.LastInsertId()
		if err != nil {
			return true, err
		}
		reflect.ValueOf(bean).Elem().FieldByName(autoIncrCol.FieldName).SetInt(id)
	}

	return n != 0, err
}

// generateInsertNoConflictSQLForMSSQL writes the INSERT ...  ON CONFLICT sql variant for MSSQL
// MSSQL uses MERGE <tablename> WITH <locK> ... but needs to pre-select the unique cols first
// then WHEN NOT MATCHED INSERT - this is kind of the opposite way round from  INSERT ... ON CONFLICT
func generateInsertNoConflictSQLForMSSQL(sb io.StringWriter, tableName string, colNames []string, args []any, uniqueCols []string, autoIncrCol *schemas.Column) {
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
		_, _ = sb.WriteString(uniqueCol)
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
	_, _ = sb.WriteString(")")
	if autoIncrCol != nil {
		_, _ = sb.WriteString(" OUTPUT INSERTED.")
		_, _ = sb.WriteString(autoIncrCol.Name)
	}
	_, _ = sb.WriteString(";")
}

func addInUniqueCols(colNames []string, args []any, emptyColNames []string, emptyArgs []any, table *schemas.Table) (uniqueCols []string, uniqueArgs []any, insertCols []string, insertArgs []any) {
	uniqueCols = make([]string, 0, len(table.Columns()))
	uniqueArgs = make([]interface{}, 1, len(uniqueCols)+1) // leave uniqueArgs[0] empty to put the SQL in
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
					// Always include empty unique columns in the insert statement
					colNames = append(colNames, col)
					args = append(args, emptyArgs[i])
					uniqueCols = append(uniqueCols, col)
					uniqueArgs = append(uniqueArgs, emptyArgs[i])
					continue indexCol
				}
			}
		}
	}
	return uniqueCols, uniqueArgs, colNames, args
}

func getColNamesAndArgsFromBean(bean interface{}, cols []*schemas.Column) (colNames []string, args []any, emptyColNames []string, emptyArgs []any, err error) {
	colNames = make([]string, len(cols))
	args = make([]any, len(cols)+1) // Leave args[0] to put the SQL in
	maxNonEmpty := 0
	minEmpty := len(cols)

	val := reflect.ValueOf(bean)
	elem := val.Elem()
	for _, col := range cols {
		if fieldIdx := col.FieldIndex; fieldIdx != nil {
			fieldVal := elem.FieldByIndex(fieldIdx)
			if col.IsCreated || col.IsUpdated {
				result, err := setCurrentTime(fieldVal, col)
				if err != nil {
					return nil, nil, nil, nil, err
				}

				colNames[maxNonEmpty] = col.Name
				maxNonEmpty++
				args[maxNonEmpty] = result
				continue
			}

			val, err := getValueFromField(fieldVal, col)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			if fieldVal.IsZero() {
				args[minEmpty] = val // remember args is 1-based not 0-based
				minEmpty--
				colNames[minEmpty] = col.Name
				continue
			}
			colNames[maxNonEmpty] = col.Name
			maxNonEmpty++
			args[maxNonEmpty] = val
		}
	}

	return colNames[:maxNonEmpty], args[:maxNonEmpty+1], colNames[maxNonEmpty:], args[maxNonEmpty+1:], nil
}

func setCurrentTime(fieldVal reflect.Value, col *schemas.Column) (interface{}, error) {
	t := time.Now()
	result, err := dialects.FormatColumnTime(x.Dialect(), x.DatabaseTZ, col, t)
	if err != nil {
		return result, err
	}

	switch fieldVal.Type().Kind() {
	case reflect.Struct:
		fieldVal.Set(reflect.ValueOf(t).Convert(fieldVal.Type()))
	case reflect.Int, reflect.Int64, reflect.Int32:
		fieldVal.SetInt(t.Unix())
	case reflect.Uint, reflect.Uint64, reflect.Uint32:
		fieldVal.SetUint(uint64(t.Unix()))
	}
	return result, nil
}

func getValueFromField(fieldVal reflect.Value, col *schemas.Column) (any, error) {
	switch fieldVal.Type().Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fieldVal.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fieldVal.Uint(), nil
	case reflect.Float32, reflect.Float64:
		return fieldVal.Float(), nil
	case reflect.Complex64, reflect.Complex128:
		return fieldVal.Complex(), nil
	case reflect.String:
		return fieldVal.String(), nil
	case reflect.Bool:
		valBool := fieldVal.Bool()

		if setting.Database.UseMSSQL {
			if valBool {
				return 1, nil
			} else {
				return 0, nil
			}
		} else {
			return valBool, nil
		}
	default:
	}

	if fieldVal.CanAddr() {
		if fieldConvert, ok := fieldVal.Addr().Interface().(convert.Conversion); ok {
			data, err := fieldConvert.ToDB()
			if err != nil {
				return nil, err
			}
			if data == nil {
				if col.Nullable {
					return nil, nil
				}
				data = []byte{}
			}
			if col.SQLType.IsBlob() {
				return data, nil
			}
			return string(data), nil
		}
	}

	isNil := fieldVal.Kind() == reflect.Ptr && fieldVal.IsNil()
	if !isNil {
		if fieldConvert, ok := fieldVal.Interface().(convert.Conversion); ok {
			data, err := fieldConvert.ToDB()
			if err != nil {
				return nil, err
			}
			if data == nil {
				if col.Nullable {
					return nil, nil
				}
				data = []byte{}
			}
			if col.SQLType.IsBlob() {
				return data, nil
			}
			return string(data), nil
		}
	}

	return fieldVal.Interface(), nil
}
