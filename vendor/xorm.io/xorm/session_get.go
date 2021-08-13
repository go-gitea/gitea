// Copyright 2016 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"time"

	"xorm.io/xorm/caches"
	"xorm.io/xorm/convert"
	"xorm.io/xorm/core"
	"xorm.io/xorm/internal/utils"
	"xorm.io/xorm/schemas"
)

var (
	// ErrObjectIsNil return error of object is nil
	ErrObjectIsNil = errors.New("object should not be nil")
)

// Get retrieve one record from database, bean's non-empty fields
// will be as conditions
func (session *Session) Get(bean interface{}) (bool, error) {
	if session.isAutoClose {
		defer session.Close()
	}
	return session.get(bean)
}

func isPtrOfTime(v interface{}) bool {
	if _, ok := v.(*time.Time); ok {
		return true
	}

	el := reflect.ValueOf(v).Elem()
	if el.Kind() != reflect.Struct {
		return false
	}

	return el.Type().ConvertibleTo(schemas.TimeType)
}

func (session *Session) get(bean interface{}) (bool, error) {
	defer session.resetStatement()

	if session.statement.LastError != nil {
		return false, session.statement.LastError
	}

	beanValue := reflect.ValueOf(bean)
	if beanValue.Kind() != reflect.Ptr {
		return false, errors.New("needs a pointer to a value")
	} else if beanValue.Elem().Kind() == reflect.Ptr {
		return false, errors.New("a pointer to a pointer is not allowed")
	} else if beanValue.IsNil() {
		return false, ErrObjectIsNil
	}

	if beanValue.Elem().Kind() == reflect.Struct && !isPtrOfTime(bean) {
		if err := session.statement.SetRefBean(bean); err != nil {
			return false, err
		}
	}

	var sqlStr string
	var args []interface{}
	var err error

	if session.statement.RawSQL == "" {
		if len(session.statement.TableName()) <= 0 {
			return false, ErrTableNotFound
		}
		session.statement.Limit(1)
		sqlStr, args, err = session.statement.GenGetSQL(bean)
		if err != nil {
			return false, err
		}
	} else {
		sqlStr = session.statement.GenRawSQL()
		args = session.statement.RawParams
	}

	table := session.statement.RefTable

	if session.statement.ColumnMap.IsEmpty() && session.canCache() && beanValue.Elem().Kind() == reflect.Struct {
		if cacher := session.engine.GetCacher(session.statement.TableName()); cacher != nil &&
			!session.statement.GetUnscoped() {
			has, err := session.cacheGet(bean, sqlStr, args...)
			if err != ErrCacheFailed {
				return has, err
			}
		}
	}

	context := session.statement.Context
	if context != nil {
		res := context.Get(fmt.Sprintf("%v-%v", sqlStr, args))
		if res != nil {
			session.engine.logger.Debugf("hit context cache: %s", sqlStr)

			structValue := reflect.Indirect(reflect.ValueOf(bean))
			structValue.Set(reflect.Indirect(reflect.ValueOf(res)))
			session.lastSQL = ""
			session.lastSQLArgs = nil
			return true, nil
		}
	}

	has, err := session.nocacheGet(beanValue.Elem().Kind(), table, bean, sqlStr, args...)
	if err != nil || !has {
		return has, err
	}

	if context != nil {
		context.Put(fmt.Sprintf("%v-%v", sqlStr, args), bean)
	}

	return true, nil
}

var (
	valuerTypePlaceHolder driver.Valuer
	valuerType            = reflect.TypeOf(&valuerTypePlaceHolder).Elem()

	scannerTypePlaceHolder sql.Scanner
	scannerType            = reflect.TypeOf(&scannerTypePlaceHolder).Elem()

	conversionTypePlaceHolder convert.Conversion
	conversionType            = reflect.TypeOf(&conversionTypePlaceHolder).Elem()
)

func isScannableStruct(bean interface{}, typeLen int) bool {
	switch bean.(type) {
	case *time.Time:
		return false
	case sql.Scanner:
		return false
	case convert.Conversion:
		return typeLen > 1
	case *big.Float:
		return false
	}
	return true
}

func (session *Session) nocacheGet(beanKind reflect.Kind, table *schemas.Table, bean interface{}, sqlStr string, args ...interface{}) (bool, error) {
	rows, err := session.queryRows(sqlStr, args...)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	if !rows.Next() {
		return false, rows.Err()
	}

	// WARN: Alougth rows return true, but we may also return error.
	types, err := rows.ColumnTypes()
	if err != nil {
		return true, err
	}
	fields, err := rows.Columns()
	if err != nil {
		return true, err
	}
	switch beanKind {
	case reflect.Struct:
		if !isScannableStruct(bean, len(types)) {
			break
		}
		return session.getStruct(rows, types, fields, table, bean)
	case reflect.Slice:
		return session.getSlice(rows, types, fields, bean)
	case reflect.Map:
		return session.getMap(rows, types, fields, bean)
	}

	return session.getVars(rows, types, fields, bean)
}

func (session *Session) getSlice(rows *core.Rows, types []*sql.ColumnType, fields []string, bean interface{}) (bool, error) {
	switch t := bean.(type) {
	case *[]string:
		res, err := session.engine.scanStringInterface(rows, fields, types)
		if err != nil {
			return true, err
		}

		var needAppend = len(*t) == 0 // both support slice is empty or has been initlized
		for i, r := range res {
			if needAppend {
				*t = append(*t, r.(*sql.NullString).String)
			} else {
				(*t)[i] = r.(*sql.NullString).String
			}
		}
		return true, nil
	case *[]interface{}:
		scanResults, err := session.engine.scanInterfaces(rows, fields, types)
		if err != nil {
			return true, err
		}
		var needAppend = len(*t) == 0
		for ii := range fields {
			s, err := convert.Interface2Interface(session.engine.DatabaseTZ, scanResults[ii])
			if err != nil {
				return true, err
			}
			if needAppend {
				*t = append(*t, s)
			} else {
				(*t)[ii] = s
			}
		}
		return true, nil
	default:
		return true, fmt.Errorf("unspoorted slice type: %t", t)
	}
}

func (session *Session) getMap(rows *core.Rows, types []*sql.ColumnType, fields []string, bean interface{}) (bool, error) {
	switch t := bean.(type) {
	case *map[string]string:
		scanResults, err := session.engine.scanStringInterface(rows, fields, types)
		if err != nil {
			return true, err
		}
		for ii, key := range fields {
			(*t)[key] = scanResults[ii].(*sql.NullString).String
		}
		return true, nil
	case *map[string]interface{}:
		scanResults, err := session.engine.scanInterfaces(rows, fields, types)
		if err != nil {
			return true, err
		}
		for ii, key := range fields {
			s, err := convert.Interface2Interface(session.engine.DatabaseTZ, scanResults[ii])
			if err != nil {
				return true, err
			}
			(*t)[key] = s
		}
		return true, nil
	default:
		return true, fmt.Errorf("unspoorted map type: %t", t)
	}
}

func (session *Session) getVars(rows *core.Rows, types []*sql.ColumnType, fields []string, beans ...interface{}) (bool, error) {
	if len(beans) != len(types) {
		return false, fmt.Errorf("expected columns %d, but only %d variables", len(types), len(beans))
	}

	err := session.engine.scan(rows, fields, types, beans...)
	return true, err
}

func (session *Session) getStruct(rows *core.Rows, types []*sql.ColumnType, fields []string, table *schemas.Table, bean interface{}) (bool, error) {
	scanResults, err := session.row2Slice(rows, fields, types, bean)
	if err != nil {
		return false, err
	}
	// close it before convert data
	rows.Close()

	dataStruct := utils.ReflectValue(bean)
	_, err = session.slice2Bean(scanResults, fields, bean, &dataStruct, table)
	if err != nil {
		return true, err
	}

	return true, session.executeProcessors()
}

func (session *Session) cacheGet(bean interface{}, sqlStr string, args ...interface{}) (has bool, err error) {
	// if has no reftable, then don't use cache currently
	if !session.canCache() {
		return false, ErrCacheFailed
	}

	for _, filter := range session.engine.dialect.Filters() {
		sqlStr = filter.Do(sqlStr)
	}
	newsql := session.statement.ConvertIDSQL(sqlStr)
	if newsql == "" {
		return false, ErrCacheFailed
	}

	tableName := session.statement.TableName()
	cacher := session.engine.cacherMgr.GetCacher(tableName)

	session.engine.logger.Debugf("[cache] Get SQL: %s, %v", newsql, args)
	table := session.statement.RefTable
	ids, err := caches.GetCacheSql(cacher, tableName, newsql, args)
	if err != nil {
		var res = make([]string, len(table.PrimaryKeys))
		rows, err := session.NoCache().queryRows(newsql, args...)
		if err != nil {
			return false, err
		}
		defer rows.Close()

		if rows.Next() {
			err = rows.ScanSlice(&res)
			if err != nil {
				return true, err
			}
		} else {
			if rows.Err() != nil {
				return false, rows.Err()
			}
			return false, ErrCacheFailed
		}

		var pk schemas.PK = make([]interface{}, len(table.PrimaryKeys))
		for i, col := range table.PKColumns() {
			if col.SQLType.IsText() {
				pk[i] = res[i]
			} else if col.SQLType.IsNumeric() {
				n, err := strconv.ParseInt(res[i], 10, 64)
				if err != nil {
					return false, err
				}
				pk[i] = n
			} else {
				return false, errors.New("unsupported")
			}
		}

		ids = []schemas.PK{pk}
		session.engine.logger.Debugf("[cache] cache ids: %s, %v", newsql, ids)
		err = caches.PutCacheSql(cacher, ids, tableName, newsql, args)
		if err != nil {
			return false, err
		}
	} else {
		session.engine.logger.Debugf("[cache] cache hit: %s, %v", newsql, ids)
	}

	if len(ids) > 0 {
		structValue := reflect.Indirect(reflect.ValueOf(bean))
		id := ids[0]
		session.engine.logger.Debugf("[cache] get bean: %s, %v", tableName, id)
		sid, err := id.ToString()
		if err != nil {
			return false, err
		}
		cacheBean := cacher.GetBean(tableName, sid)
		if cacheBean == nil {
			cacheBean = bean
			has, err = session.nocacheGet(reflect.Struct, table, cacheBean, sqlStr, args...)
			if err != nil || !has {
				return has, err
			}

			session.engine.logger.Debugf("[cache] cache bean: %s, %v, %v", tableName, id, cacheBean)
			cacher.PutBean(tableName, sid, cacheBean)
		} else {
			session.engine.logger.Debugf("[cache] cache hit: %s, %v, %v", tableName, id, cacheBean)
			has = true
		}
		structValue.Set(reflect.Indirect(reflect.ValueOf(cacheBean)))

		return has, nil
	}
	return false, nil
}
