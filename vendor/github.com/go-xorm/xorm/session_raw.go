// Copyright 2016 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/go-xorm/core"
)

func (session *Session) query(sqlStr string, paramStr ...interface{}) ([]map[string][]byte, error) {
	session.queryPreprocess(&sqlStr, paramStr...)

	if session.isAutoCommit {
		return session.innerQuery2(sqlStr, paramStr...)
	}
	return session.txQuery(session.tx, sqlStr, paramStr...)
}

func (session *Session) txQuery(tx *core.Tx, sqlStr string, params ...interface{}) ([]map[string][]byte, error) {
	rows, err := tx.Query(sqlStr, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return rows2maps(rows)
}

func (session *Session) innerQuery(sqlStr string, params ...interface{}) (*core.Stmt, *core.Rows, error) {
	var callback func() (*core.Stmt, *core.Rows, error)
	if session.prepareStmt {
		callback = func() (*core.Stmt, *core.Rows, error) {
			stmt, err := session.doPrepare(sqlStr)
			if err != nil {
				return nil, nil, err
			}
			rows, err := stmt.Query(params...)
			if err != nil {
				return nil, nil, err
			}
			return stmt, rows, nil
		}
	} else {
		callback = func() (*core.Stmt, *core.Rows, error) {
			rows, err := session.DB().Query(sqlStr, params...)
			if err != nil {
				return nil, nil, err
			}
			return nil, rows, err
		}
	}
	stmt, rows, err := session.engine.logSQLQueryTime(sqlStr, params, callback)
	if err != nil {
		return nil, nil, err
	}
	return stmt, rows, nil
}

func rows2maps(rows *core.Rows) (resultsSlice []map[string][]byte, err error) {
	fields, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		result, err := row2map(rows, fields)
		if err != nil {
			return nil, err
		}
		resultsSlice = append(resultsSlice, result)
	}

	return resultsSlice, nil
}

func value2Bytes(rawValue *reflect.Value) (data []byte, err error) {
	var str string
	str, err = reflect2value(rawValue)
	if err != nil {
		return
	}
	data = []byte(str)
	return
}

func row2map(rows *core.Rows, fields []string) (resultsMap map[string][]byte, err error) {
	result := make(map[string][]byte)
	scanResultContainers := make([]interface{}, len(fields))
	for i := 0; i < len(fields); i++ {
		var scanResultContainer interface{}
		scanResultContainers[i] = &scanResultContainer
	}
	if err := rows.Scan(scanResultContainers...); err != nil {
		return nil, err
	}

	for ii, key := range fields {
		rawValue := reflect.Indirect(reflect.ValueOf(scanResultContainers[ii]))
		//if row is null then ignore
		if rawValue.Interface() == nil {
			//fmt.Println("ignore ...", key, rawValue)
			continue
		}

		if data, err := value2Bytes(&rawValue); err == nil {
			result[key] = data
		} else {
			return nil, err // !nashtsai! REVIEW, should return err or just error log?
		}
	}
	return result, nil
}

func (session *Session) innerQuery2(sqlStr string, params ...interface{}) ([]map[string][]byte, error) {
	_, rows, err := session.innerQuery(sqlStr, params...)
	if rows != nil {
		defer rows.Close()
	}
	if err != nil {
		return nil, err
	}
	return rows2maps(rows)
}

// Query runs a raw sql and return records as []map[string][]byte
func (session *Session) Query(sqlStr string, paramStr ...interface{}) ([]map[string][]byte, error) {
	defer session.resetStatement()
	if session.isAutoClose {
		defer session.Close()
	}

	return session.query(sqlStr, paramStr...)
}

func rows2Strings(rows *core.Rows) (resultsSlice []map[string]string, err error) {
	fields, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		result, err := row2mapStr(rows, fields)
		if err != nil {
			return nil, err
		}
		resultsSlice = append(resultsSlice, result)
	}

	return resultsSlice, nil
}

func reflect2value(rawValue *reflect.Value) (str string, err error) {
	aa := reflect.TypeOf((*rawValue).Interface())
	vv := reflect.ValueOf((*rawValue).Interface())
	switch aa.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		str = strconv.FormatInt(vv.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		str = strconv.FormatUint(vv.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		str = strconv.FormatFloat(vv.Float(), 'f', -1, 64)
	case reflect.String:
		str = vv.String()
	case reflect.Array, reflect.Slice:
		switch aa.Elem().Kind() {
		case reflect.Uint8:
			data := rawValue.Interface().([]byte)
			str = string(data)
		default:
			err = fmt.Errorf("Unsupported struct type %v", vv.Type().Name())
		}
	// time type
	case reflect.Struct:
		if aa.ConvertibleTo(core.TimeType) {
			str = vv.Convert(core.TimeType).Interface().(time.Time).Format(time.RFC3339Nano)
		} else {
			err = fmt.Errorf("Unsupported struct type %v", vv.Type().Name())
		}
	case reflect.Bool:
		str = strconv.FormatBool(vv.Bool())
	case reflect.Complex128, reflect.Complex64:
		str = fmt.Sprintf("%v", vv.Complex())
	/* TODO: unsupported types below
	   case reflect.Map:
	   case reflect.Ptr:
	   case reflect.Uintptr:
	   case reflect.UnsafePointer:
	   case reflect.Chan, reflect.Func, reflect.Interface:
	*/
	default:
		err = fmt.Errorf("Unsupported struct type %v", vv.Type().Name())
	}
	return
}

func value2String(rawValue *reflect.Value) (data string, err error) {
	data, err = reflect2value(rawValue)
	if err != nil {
		return
	}
	return
}

func row2mapStr(rows *core.Rows, fields []string) (resultsMap map[string]string, err error) {
	result := make(map[string]string)
	scanResultContainers := make([]interface{}, len(fields))
	for i := 0; i < len(fields); i++ {
		var scanResultContainer interface{}
		scanResultContainers[i] = &scanResultContainer
	}
	if err := rows.Scan(scanResultContainers...); err != nil {
		return nil, err
	}

	for ii, key := range fields {
		rawValue := reflect.Indirect(reflect.ValueOf(scanResultContainers[ii]))
		//if row is null then ignore
		if rawValue.Interface() == nil {
			//fmt.Println("ignore ...", key, rawValue)
			continue
		}

		if data, err := value2String(&rawValue); err == nil {
			result[key] = data
		} else {
			return nil, err // !nashtsai! REVIEW, should return err or just error log?
		}
	}
	return result, nil
}

func txQuery2(tx *core.Tx, sqlStr string, params ...interface{}) ([]map[string]string, error) {
	rows, err := tx.Query(sqlStr, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return rows2Strings(rows)
}

func query2(db *core.DB, sqlStr string, params ...interface{}) ([]map[string]string, error) {
	rows, err := db.Query(sqlStr, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return rows2Strings(rows)
}

// QueryString runs a raw sql and return records as []map[string]string
func (session *Session) QueryString(sqlStr string, args ...interface{}) ([]map[string]string, error) {
	defer session.resetStatement()
	if session.isAutoClose {
		defer session.Close()
	}

	session.queryPreprocess(&sqlStr, args...)

	if session.isAutoCommit {
		return query2(session.DB(), sqlStr, args...)
	}
	return txQuery2(session.tx, sqlStr, args...)
}

// Execute sql
func (session *Session) innerExec(sqlStr string, args ...interface{}) (sql.Result, error) {
	if session.prepareStmt {
		stmt, err := session.doPrepare(sqlStr)
		if err != nil {
			return nil, err
		}

		res, err := stmt.Exec(args...)
		if err != nil {
			return nil, err
		}
		return res, nil
	}

	return session.DB().Exec(sqlStr, args...)
}

func (session *Session) exec(sqlStr string, args ...interface{}) (sql.Result, error) {
	for _, filter := range session.engine.dialect.Filters() {
		// TODO: for table name, it's no need to RefTable
		sqlStr = filter.Do(sqlStr, session.engine.dialect, session.statement.RefTable)
	}

	session.saveLastSQL(sqlStr, args...)

	return session.engine.logSQLExecutionTime(sqlStr, args, func() (sql.Result, error) {
		if session.isAutoCommit {
			// FIXME: oci8 can not auto commit (github.com/mattn/go-oci8)
			if session.engine.dialect.DBType() == core.ORACLE {
				session.Begin()
				r, err := session.tx.Exec(sqlStr, args...)
				session.Commit()
				return r, err
			}
			return session.innerExec(sqlStr, args...)
		}
		return session.tx.Exec(sqlStr, args...)
	})
}

// Exec raw sql
func (session *Session) Exec(sqlStr string, args ...interface{}) (sql.Result, error) {
	defer session.resetStatement()
	if session.isAutoClose {
		defer session.Close()
	}

	return session.exec(sqlStr, args...)
}
