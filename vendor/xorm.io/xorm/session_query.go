// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"xorm.io/xorm/core"
)

// Query runs a raw sql and return records as []map[string][]byte
func (session *Session) Query(sqlOrArgs ...interface{}) ([]map[string][]byte, error) {
	if session.isAutoClose {
		defer session.Close()
	}

	sqlStr, args, err := session.statement.GenQuerySQL(sqlOrArgs...)
	if err != nil {
		return nil, err
	}

	return session.queryBytes(sqlStr, args...)
}

func (session *Session) rows2Strings(rows *core.Rows) (resultsSlice []map[string]string, err error) {
	fields, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		result, err := session.engine.row2mapStr(rows, types, fields)
		if err != nil {
			return nil, err
		}
		resultsSlice = append(resultsSlice, result)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return resultsSlice, nil
}

func (session *Session) rows2SliceString(rows *core.Rows) (resultsSlice [][]string, err error) {
	fields, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		record, err := session.engine.row2sliceStr(rows, types, fields)
		if err != nil {
			return nil, err
		}
		resultsSlice = append(resultsSlice, record)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return resultsSlice, nil
}

// QueryString runs a raw sql and return records as []map[string]string
func (session *Session) QueryString(sqlOrArgs ...interface{}) ([]map[string]string, error) {
	if session.isAutoClose {
		defer session.Close()
	}

	sqlStr, args, err := session.statement.GenQuerySQL(sqlOrArgs...)
	if err != nil {
		return nil, err
	}

	rows, err := session.queryRows(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return session.rows2Strings(rows)
}

// QuerySliceString runs a raw sql and return records as [][]string
func (session *Session) QuerySliceString(sqlOrArgs ...interface{}) ([][]string, error) {
	if session.isAutoClose {
		defer session.Close()
	}

	sqlStr, args, err := session.statement.GenQuerySQL(sqlOrArgs...)
	if err != nil {
		return nil, err
	}

	rows, err := session.queryRows(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return session.rows2SliceString(rows)
}

func (session *Session) rows2Interfaces(rows *core.Rows) (resultsSlice []map[string]interface{}, err error) {
	fields, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		result, err := session.engine.row2mapInterface(rows, types, fields)
		if err != nil {
			return nil, err
		}
		resultsSlice = append(resultsSlice, result)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return resultsSlice, nil
}

// QueryInterface runs a raw sql and return records as []map[string]interface{}
func (session *Session) QueryInterface(sqlOrArgs ...interface{}) ([]map[string]interface{}, error) {
	if session.isAutoClose {
		defer session.Close()
	}

	sqlStr, args, err := session.statement.GenQuerySQL(sqlOrArgs...)
	if err != nil {
		return nil, err
	}

	rows, err := session.queryRows(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return session.rows2Interfaces(rows)
}
