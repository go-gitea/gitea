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
	if setting.Database.Type.IsSQLite3() {
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

	columns := table.Columns()

	colNames, values, zeroedColNames, zeroedValues, err := getColNamesAndValuesFromBean(bean, columns)
	if err != nil {
		return false, err
	}

	if len(colNames) == 0 {
		return false, fmt.Errorf("provided bean to insert has all empty values")
	}

	// MSSQL needs to separately pass in the columns with the unique constraint and we need to
	// include empty columns which are in the constraint in the insert for other dbs
	uniqueColValMap, colNames, values := addInUniqueCols(colNames, values, zeroedColNames, zeroedValues, table)
	if len(uniqueColValMap) == 0 {
		return false, fmt.Errorf("provided bean has no unique constraints")
	}

	var insertArgs []any

	switch {
	case setting.Database.Type.IsSQLite3():
		insertArgs = generateInsertNoConflictSQLAndArgsForSQLite(tableName, colNames, values)
	case setting.Database.Type.IsPostgreSQL():
		insertArgs = generateInsertNoConflictSQLAndArgsForPostgres(tableName, colNames, values, autoIncrCol)
	case setting.Database.Type.IsMySQL():
		insertArgs = generateInsertNoConflictSQLAndArgsForMySQL(tableName, colNames, values)
	case setting.Database.Type.IsMSSQL():
		insertArgs = generateInsertNoConflictSQLAndArgsForMSSQL(table, tableName, colNames, values, uniqueColValMap, autoIncrCol)
	default:
		return false, fmt.Errorf("database type not supported")
	}

	if autoIncrCol != nil && (setting.Database.Type.IsPostgreSQL() || setting.Database.Type.IsMSSQL()) {
		// Postgres and MSSQL do not use the LastInsertID mechanism
		// Therefore use query rather than exec and read the last provided ID back in

		res, err := e.Query(insertArgs...)
		if err != nil {
			return false, fmt.Errorf("error in query: %s, %w", insertArgs[0], err)
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

	res, err := e.Exec(insertArgs...)
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

// generateInsertNoConflictSQLAndArgsForSQLite will create the correct insert code for SQLite
func generateInsertNoConflictSQLAndArgsForSQLite(tableName string, colNames []string, args []any) (insertArgs []any) {
	sb := &strings.Builder{}

	quote := x.Dialect().Quoter().Quote
	write := func(args ...string) {
		for _, arg := range args {
			_, _ = sb.WriteString(arg)
		}
	}
	write("INSERT INTO ", quote(tableName), " (")
	_ = x.Dialect().Quoter().JoinWrite(sb, colNames, ",")
	write(") VALUES (?")
	for range colNames[1:] {
		write(",?")
	}
	write(") ON CONFLICT DO NOTHING")
	args[0] = sb.String()
	return args
}

// generateInsertNoConflictSQLAndArgsForPostgres will create the correct insert code for Postgres
func generateInsertNoConflictSQLAndArgsForPostgres(tableName string, colNames []string, args []any, autoIncrCol *schemas.Column) (insertArgs []any) {
	sb := &strings.Builder{}

	quote := x.Dialect().Quoter().Quote
	write := func(args ...string) {
		for _, arg := range args {
			_, _ = sb.WriteString(arg)
		}
	}
	write("INSERT INTO ", quote(tableName), " (")
	_ = x.Dialect().Quoter().JoinWrite(sb, colNames, ",")
	write(") VALUES (?")
	for range colNames[1:] {
		write(",?")
	}
	write(") ON CONFLICT DO NOTHING")
	if autoIncrCol != nil {
		write(" RETURNING ", quote(autoIncrCol.Name))
	}
	args[0] = sb.String()
	return args
}

// generateInsertNoConflictSQLAndArgsForMySQL will create the correct insert code for MySQL
func generateInsertNoConflictSQLAndArgsForMySQL(tableName string, colNames []string, args []any) (insertArgs []any) {
	sb := &strings.Builder{}

	quote := x.Dialect().Quoter().Quote
	write := func(args ...string) {
		for _, arg := range args {
			_, _ = sb.WriteString(arg)
		}
	}
	write("INSERT IGNORE INTO ", quote(tableName), " (")
	_ = x.Dialect().Quoter().JoinWrite(sb, colNames, ",")
	write(") VALUES (?")
	for range colNames[1:] {
		write(",?")
	}
	write(")")
	args[0] = sb.String()
	return args
}

// generateInsertNoConflictSQLAndArgsForMSSQL writes the INSERT ...  ON CONFLICT sql variant for MSSQL
// MSSQL uses MERGE <tablename> WITH <lock> ... but needs to pre-select the unique cols first
// then WHEN NOT MATCHED INSERT - this is kind of the opposite way round from  INSERT ... ON CONFLICT
func generateInsertNoConflictSQLAndArgsForMSSQL(table *schemas.Table, tableName string, colNames []string, args []any, uniqueColValMap map[string]any, autoIncrCol *schemas.Column) (insertArgs []any) {
	sb := &strings.Builder{}

	quote := x.Dialect().Quoter().Quote
	write := func(args ...string) {
		for _, arg := range args {
			_, _ = sb.WriteString(arg)
		}
	}
	uniqueCols := make([]string, 0, len(uniqueColValMap))
	for colName := range uniqueColValMap {
		uniqueCols = append(uniqueCols, colName)
	}

	write("MERGE ", quote(tableName), " WITH (HOLDLOCK) AS target USING (SELECT ? AS ")
	_ = x.Dialect().Quoter().JoinWrite(sb, uniqueCols, ", ? AS ")
	write(") AS src ON (")
	countUniques := 0
	for _, index := range table.Indexes {
		if index.Type != schemas.UniqueType {
			continue
		}
		if countUniques > 0 {
			write(" OR ")
		}
		countUniques++
		write("(")
		write("src.", quote(index.Cols[0]), "= target.", quote(index.Cols[0]))
		for _, col := range index.Cols[1:] {
			write(" AND src.", quote(col), "= target.", quote(col))
		}
		write(")")
	}
	write(") WHEN NOT MATCHED THEN INSERT (")
	_ = x.Dialect().Quoter().JoinWrite(sb, colNames, ",")
	write(") VALUES (?")
	for range colNames[1:] {
		write(", ?")
	}
	write(")")
	if autoIncrCol != nil {
		write(" OUTPUT INSERTED.", quote(autoIncrCol.Name))
	}
	write(";")
	uniqueArgs := make([]any, 0, len(uniqueColValMap)+len(args))
	uniqueArgs = append(uniqueArgs, sb.String())
	for _, col := range uniqueCols {
		uniqueArgs = append(uniqueArgs, uniqueColValMap[col])
	}
	return append(uniqueArgs, args[1:]...)
}

// addInUniqueCols determines the columns that refer to unique constraints and creates slices for these
// as they're needed by MSSQL. In addition, any columns which are zero-valued but are part of a constraint
// are added back in to the colNames and args
func addInUniqueCols(colNames []string, args []any, zeroedColNames []string, emptyArgs []any, table *schemas.Table) (uniqueColValMap map[string]any, insertCols []string, insertArgs []any) {
	uniqueColValMap = make(map[string]any)

	// Iterate across the indexes in the provided table
	for _, index := range table.Indexes {
		if index.Type != schemas.UniqueType {
			continue
		}

		// index is a Unique constraint
	indexCol:
		for _, iCol := range index.Cols {
			if _, has := uniqueColValMap[iCol]; has {
				// column is already included in uniqueCols so we don't need to add it again
				continue indexCol
			}

			// Now iterate across colNames and add to the uniqueCols
			for i, col := range colNames {
				if col == iCol {
					uniqueColValMap[col] = args[i+1]
					continue indexCol
				}
			}

			// If we still haven't found the column we need to look in the emptyColumns and add
			// it back into colNames and args as well as uniqueCols/uniqueArgs
			for i, col := range zeroedColNames {
				if col == iCol {
					// Always include empty unique columns in the insert statement as otherwise the insert no conflict will pass
					colNames = append(colNames, col)
					args = append(args, emptyArgs[i])
					uniqueColValMap[col] = emptyArgs[i]
					continue indexCol
				}
			}
		}
	}
	return uniqueColValMap, colNames, args
}

// getColNamesAndValuesFromBean reads the provided bean, providing two pairs of linked slices:
//
// - colNames and values
// - zeroedColNames and zeroedValues
//
// colNames contains the names of the columns that have non-zero values in the provided bean
// values contains the values - with one exception - values is 1-based so that values[0] is deliberately left zero
//
// emptyyColNames and zeroedValues accounts for the other columns - with zeroedValues containing the zero values
func getColNamesAndValuesFromBean(bean interface{}, cols []*schemas.Column) (colNames []string, values []any, zeroedColNames []string, zeroedValues []any, err error) {
	colNames = make([]string, len(cols))
	values = make([]any, len(cols)+1) // Leave args[0] to put the SQL in
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
				values[maxNonEmpty] = result
				continue
			}

			val, err := getValueFromField(fieldVal, col)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			if fieldVal.IsZero() {
				values[minEmpty] = val // remember args is 1-based not 0-based
				minEmpty--
				colNames[minEmpty] = col.Name
				continue
			}
			colNames[maxNonEmpty] = col.Name
			maxNonEmpty++
			values[maxNonEmpty] = val
		}
	}

	return colNames[:maxNonEmpty], values[:maxNonEmpty+1], colNames[maxNonEmpty:], values[maxNonEmpty+1:], nil
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

// getValueFromField extracts the reflected value from the provided fieldVal
// this keeps the type and makes such that zero values work in the SQL Insert above
func getValueFromField(fieldVal reflect.Value, col *schemas.Column) (any, error) {
	// Handle pointers to convert.Conversion
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

	// Handle nil pointer to convert.Conversion
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

	// Handle common primitive types
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

		if setting.Database.Type.IsMSSQL() {
			if valBool {
				return 1, nil
			}
			return 0, nil
		}
		return valBool, nil
	default:
	}

	// just return the interface
	return fieldVal.Interface(), nil
}
