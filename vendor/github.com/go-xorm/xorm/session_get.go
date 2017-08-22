// Copyright 2016 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"errors"
	"reflect"
	"strconv"

	"github.com/go-xorm/core"
)

// Get retrieve one record from database, bean's non-empty fields
// will be as conditions
func (session *Session) Get(bean interface{}) (bool, error) {
	defer session.resetStatement()
	if session.isAutoClose {
		defer session.Close()
	}

	beanValue := reflect.ValueOf(bean)
	if beanValue.Kind() != reflect.Ptr {
		return false, errors.New("needs a pointer to a value")
	} else if beanValue.Elem().Kind() == reflect.Ptr {
		return false, errors.New("a pointer to a pointer is not allowed")
	}

	if beanValue.Elem().Kind() == reflect.Struct {
		if err := session.statement.setRefValue(beanValue.Elem()); err != nil {
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
		sqlStr, args, err = session.statement.genGetSQL(bean)
		if err != nil {
			return false, err
		}
	} else {
		sqlStr = session.statement.RawSQL
		args = session.statement.RawParams
	}

	if session.canCache() && beanValue.Elem().Kind() == reflect.Struct {
		if cacher := session.engine.getCacher2(session.statement.RefTable); cacher != nil &&
			!session.statement.unscoped {
			has, err := session.cacheGet(bean, sqlStr, args...)
			if err != ErrCacheFailed {
				return has, err
			}
		}
	}

	return session.nocacheGet(beanValue.Elem().Kind(), bean, sqlStr, args...)
}

func (session *Session) nocacheGet(beanKind reflect.Kind, bean interface{}, sqlStr string, args ...interface{}) (bool, error) {
	session.queryPreprocess(&sqlStr, args...)

	var rawRows *core.Rows
	var err error
	if session.isAutoCommit {
		_, rawRows, err = session.innerQuery(sqlStr, args...)
	} else {
		rawRows, err = session.tx.Query(sqlStr, args...)
	}
	if err != nil {
		return false, err
	}

	defer rawRows.Close()

	if !rawRows.Next() {
		return false, nil
	}

	switch beanKind {
	case reflect.Struct:
		fields, err := rawRows.Columns()
		if err != nil {
			// WARN: Alougth rawRows return true, but get fields failed
			return true, err
		}
		dataStruct := rValue(bean)
		if err := session.statement.setRefValue(dataStruct); err != nil {
			return false, err
		}

		scanResults, err := session.row2Slice(rawRows, fields, len(fields), bean)
		if err != nil {
			return false, err
		}
		rawRows.Close()

		_, err = session.slice2Bean(scanResults, fields, len(fields), bean, &dataStruct, session.statement.RefTable)
	case reflect.Slice:
		err = rawRows.ScanSlice(bean)
	case reflect.Map:
		err = rawRows.ScanMap(bean)
	default:
		err = rawRows.Scan(bean)
	}

	return true, err
}

func (session *Session) cacheGet(bean interface{}, sqlStr string, args ...interface{}) (has bool, err error) {
	// if has no reftable, then don't use cache currently
	if !session.canCache() {
		return false, ErrCacheFailed
	}

	for _, filter := range session.engine.dialect.Filters() {
		sqlStr = filter.Do(sqlStr, session.engine.dialect, session.statement.RefTable)
	}
	newsql := session.statement.convertIDSQL(sqlStr)
	if newsql == "" {
		return false, ErrCacheFailed
	}

	cacher := session.engine.getCacher2(session.statement.RefTable)
	tableName := session.statement.TableName()
	session.engine.logger.Debug("[cacheGet] find sql:", newsql, args)
	ids, err := core.GetCacheSql(cacher, tableName, newsql, args)
	table := session.statement.RefTable
	if err != nil {
		var res = make([]string, len(table.PrimaryKeys))
		rows, err := session.DB().Query(newsql, args...)
		if err != nil {
			return false, err
		}
		defer rows.Close()

		if rows.Next() {
			err = rows.ScanSlice(&res)
			if err != nil {
				return false, err
			}
		} else {
			return false, ErrCacheFailed
		}

		var pk core.PK = make([]interface{}, len(table.PrimaryKeys))
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

		ids = []core.PK{pk}
		session.engine.logger.Debug("[cacheGet] cache ids:", newsql, ids)
		err = core.PutCacheSql(cacher, ids, tableName, newsql, args)
		if err != nil {
			return false, err
		}
	} else {
		session.engine.logger.Debug("[cacheGet] cache hit sql:", newsql)
	}

	if len(ids) > 0 {
		structValue := reflect.Indirect(reflect.ValueOf(bean))
		id := ids[0]
		session.engine.logger.Debug("[cacheGet] get bean:", tableName, id)
		sid, err := id.ToString()
		if err != nil {
			return false, err
		}
		cacheBean := cacher.GetBean(tableName, sid)
		if cacheBean == nil {
			cacheBean = bean
			has, err = session.nocacheGet(reflect.Struct, cacheBean, sqlStr, args...)
			if err != nil || !has {
				return has, err
			}

			session.engine.logger.Debug("[cacheGet] cache bean:", tableName, id, cacheBean)
			cacher.PutBean(tableName, sid, cacheBean)
		} else {
			session.engine.logger.Debug("[cacheGet] cache hit bean:", tableName, id, cacheBean)
			has = true
		}
		structValue.Set(reflect.Indirect(reflect.ValueOf(cacheBean)))

		return has, nil
	}
	return false, nil
}
