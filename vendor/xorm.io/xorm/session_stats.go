// Copyright 2016 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"database/sql"
	"errors"
	"reflect"
)

// Count counts the records. bean's non-empty fields
// are conditions.
func (session *Session) Count(bean ...interface{}) (int64, error) {
	if session.isAutoClose {
		defer session.Close()
	}

	sqlStr, args, err := session.statement.GenCountSQL(bean...)
	if err != nil {
		return 0, err
	}

	var total int64
	err = session.queryRow(sqlStr, args...).Scan(&total)
	if err == sql.ErrNoRows || err == nil {
		return total, nil
	}

	return 0, err
}

// sum call sum some column. bean's non-empty fields are conditions.
func (session *Session) sum(res interface{}, bean interface{}, columnNames ...string) error {
	if session.isAutoClose {
		defer session.Close()
	}

	v := reflect.ValueOf(res)
	if v.Kind() != reflect.Ptr {
		return errors.New("need a pointer to a variable")
	}

	sqlStr, args, err := session.statement.GenSumSQL(bean, columnNames...)
	if err != nil {
		return err
	}

	if v.Elem().Kind() == reflect.Slice {
		err = session.queryRow(sqlStr, args...).ScanSlice(res)
	} else {
		err = session.queryRow(sqlStr, args...).Scan(res)
	}
	if err == sql.ErrNoRows || err == nil {
		return nil
	}
	return err
}

// Sum call sum some column. bean's non-empty fields are conditions.
func (session *Session) Sum(bean interface{}, columnName string) (res float64, err error) {
	return res, session.sum(&res, bean, columnName)
}

// SumInt call sum some column. bean's non-empty fields are conditions.
func (session *Session) SumInt(bean interface{}, columnName string) (res int64, err error) {
	return res, session.sum(&res, bean, columnName)
}

// Sums call sum some columns. bean's non-empty fields are conditions.
func (session *Session) Sums(bean interface{}, columnNames ...string) ([]float64, error) {
	var res = make([]float64, len(columnNames))
	return res, session.sum(&res, bean, columnNames...)
}

// SumsInt sum specify columns and return as []int64 instead of []float64
func (session *Session) SumsInt(bean interface{}, columnNames ...string) ([]int64, error) {
	var res = make([]int64, len(columnNames))
	return res, session.sum(&res, bean, columnNames...)
}
