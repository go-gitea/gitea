// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"errors"
	"fmt"
	"reflect"

	"xorm.io/builder"
	"xorm.io/xorm/core"
	"xorm.io/xorm/internal/utils"
)

// Rows rows wrapper a rows to
type Rows struct {
	session  *Session
	rows     *core.Rows
	beanType reflect.Type
}

func newRows(session *Session, bean interface{}) (*Rows, error) {
	rows := new(Rows)
	rows.session = session
	rows.beanType = reflect.Indirect(reflect.ValueOf(bean)).Type()

	var sqlStr string
	var args []interface{}
	var err error

	beanValue := reflect.ValueOf(bean)
	if beanValue.Kind() != reflect.Ptr {
		return nil, errors.New("needs a pointer to a value")
	} else if beanValue.Elem().Kind() == reflect.Ptr {
		return nil, errors.New("a pointer to a pointer is not allowed")
	}

	if err = rows.session.statement.SetRefBean(bean); err != nil {
		return nil, err
	}

	if len(session.statement.TableName()) <= 0 {
		return nil, ErrTableNotFound
	}

	if rows.session.statement.RawSQL == "" {
		var autoCond builder.Cond
		var addedTableName = (len(session.statement.JoinStr) > 0)
		var table = rows.session.statement.RefTable

		if !session.statement.NoAutoCondition {
			var err error
			autoCond, err = session.statement.BuildConds(table, bean, true, true, false, true, addedTableName)
			if err != nil {
				return nil, err
			}
		} else {
			// !oinume! Add "<col> IS NULL" to WHERE whatever condiBean is given.
			// See https://gitea.com/xorm/xorm/issues/179
			if col := table.DeletedColumn(); col != nil && !session.statement.GetUnscoped() { // tag "deleted" is enabled
				autoCond = session.statement.CondDeleted(col)
			}
		}

		sqlStr, args, err = rows.session.statement.GenFindSQL(autoCond)
		if err != nil {
			return nil, err
		}
	} else {
		sqlStr = rows.session.statement.GenRawSQL()
		args = rows.session.statement.RawParams
	}

	rows.rows, err = rows.session.queryRows(sqlStr, args...)
	if err != nil {
		rows.Close()
		return nil, err
	}

	return rows, nil
}

// Next move cursor to next record, return false if end has reached
func (rows *Rows) Next() bool {
	if rows.rows != nil {
		return rows.rows.Next()
	}
	return false
}

// Err returns the error, if any, that was encountered during iteration. Err may be called after an explicit or implicit Close.
func (rows *Rows) Err() error {
	if rows.rows != nil {
		return rows.rows.Err()
	}
	return nil
}

// Scan row record to bean properties
func (rows *Rows) Scan(bean interface{}) error {
	if rows.Err() != nil {
		return rows.Err()
	}

	if reflect.Indirect(reflect.ValueOf(bean)).Type() != rows.beanType {
		return fmt.Errorf("scan arg is incompatible type to [%v]", rows.beanType)
	}

	if err := rows.session.statement.SetRefBean(bean); err != nil {
		return err
	}

	fields, err := rows.rows.Columns()
	if err != nil {
		return err
	}
	types, err := rows.rows.ColumnTypes()
	if err != nil {
		return err
	}

	scanResults, err := rows.session.row2Slice(rows.rows, fields, types, bean)
	if err != nil {
		return err
	}

	dataStruct := utils.ReflectValue(bean)
	_, err = rows.session.slice2Bean(scanResults, fields, bean, &dataStruct, rows.session.statement.RefTable)
	if err != nil {
		return err
	}

	return rows.session.executeProcessors()
}

// Close session if session.IsAutoClose is true, and claimed any opened resources
func (rows *Rows) Close() error {
	if rows.session.isAutoClose {
		defer rows.session.Close()
	}

	if rows.rows != nil {
		return rows.rows.Close()
	}

	return rows.Err()
}
