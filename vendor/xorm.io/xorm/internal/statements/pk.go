// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package statements

import (
	"fmt"
	"reflect"

	"xorm.io/builder"
	"xorm.io/xorm/schemas"
)

var (
	ptrPkType  = reflect.TypeOf(&schemas.PK{})
	pkType     = reflect.TypeOf(schemas.PK{})
	stringType = reflect.TypeOf("")
	intType    = reflect.TypeOf(int64(0))
	uintType   = reflect.TypeOf(uint64(0))
)

// ErrIDConditionWithNoTable represents an error there is no reference table with an ID condition
type ErrIDConditionWithNoTable struct {
	ID schemas.PK
}

func (err ErrIDConditionWithNoTable) Error() string {
	return fmt.Sprintf("ID condition %#v need reference table", err.ID)
}

// IsIDConditionWithNoTableErr return true if the err is ErrIDConditionWithNoTable
func IsIDConditionWithNoTableErr(err error) bool {
	_, ok := err.(ErrIDConditionWithNoTable)
	return ok
}

// ID generate "where id = ? " statement or for composite key "where key1 = ? and key2 = ?"
func (statement *Statement) ID(id interface{}) *Statement {
	switch t := id.(type) {
	case *schemas.PK:
		statement.idParam = *t
	case schemas.PK:
		statement.idParam = t
	case string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		statement.idParam = schemas.PK{id}
	default:
		idValue := reflect.ValueOf(id)
		idType := idValue.Type()

		switch idType.Kind() {
		case reflect.String:
			statement.idParam = schemas.PK{idValue.Convert(stringType).Interface()}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			statement.idParam = schemas.PK{idValue.Convert(intType).Interface()}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			statement.idParam = schemas.PK{idValue.Convert(uintType).Interface()}
		case reflect.Slice:
			if idType.ConvertibleTo(pkType) {
				statement.idParam = idValue.Convert(pkType).Interface().(schemas.PK)
			}
		case reflect.Ptr:
			if idType.ConvertibleTo(ptrPkType) {
				statement.idParam = idValue.Convert(ptrPkType).Elem().Interface().(schemas.PK)
			}
		}
	}

	if statement.idParam == nil {
		statement.LastError = fmt.Errorf("ID param %#v is not supported", id)
	}

	return statement
}

// ProcessIDParam handles the process of id condition
func (statement *Statement) ProcessIDParam() error {
	if statement.idParam == nil {
		return nil
	}

	if statement.RefTable == nil {
		return ErrIDConditionWithNoTable{statement.idParam}
	}

	if len(statement.RefTable.PrimaryKeys) != len(statement.idParam) {
		return fmt.Errorf("ID condition is error, expect %d primarykeys, there are %d",
			len(statement.RefTable.PrimaryKeys),
			len(statement.idParam),
		)
	}

	for i, col := range statement.RefTable.PKColumns() {
		var colName = statement.colName(col, statement.TableName())
		statement.cond = statement.cond.And(builder.Eq{colName: statement.idParam[i]})
	}
	return nil
}
