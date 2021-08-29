// Copyright 2016 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"errors"
	"fmt"
	"strconv"

	"xorm.io/xorm/caches"
	"xorm.io/xorm/schemas"
)

var (
	// ErrNeedDeletedCond delete needs less one condition error
	ErrNeedDeletedCond = errors.New("Delete action needs at least one condition")

	// ErrNotImplemented not implemented
	ErrNotImplemented = errors.New("Not implemented")
)

func (session *Session) cacheDelete(table *schemas.Table, tableName, sqlStr string, args ...interface{}) error {
	if table == nil ||
		session.tx != nil {
		return ErrCacheFailed
	}

	for _, filter := range session.engine.dialect.Filters() {
		sqlStr = filter.Do(sqlStr)
	}

	newsql := session.statement.ConvertIDSQL(sqlStr)
	if newsql == "" {
		return ErrCacheFailed
	}

	cacher := session.engine.cacherMgr.GetCacher(tableName)
	pkColumns := table.PKColumns()
	ids, err := caches.GetCacheSql(cacher, tableName, newsql, args)
	if err != nil {
		resultsSlice, err := session.queryBytes(newsql, args...)
		if err != nil {
			return err
		}
		ids = make([]schemas.PK, 0)
		if len(resultsSlice) > 0 {
			for _, data := range resultsSlice {
				var id int64
				var pk schemas.PK = make([]interface{}, 0)
				for _, col := range pkColumns {
					if v, ok := data[col.Name]; !ok {
						return errors.New("no id")
					} else if col.SQLType.IsText() {
						pk = append(pk, string(v))
					} else if col.SQLType.IsNumeric() {
						id, err = strconv.ParseInt(string(v), 10, 64)
						if err != nil {
							return err
						}
						pk = append(pk, id)
					} else {
						return errors.New("not supported primary key type")
					}
				}
				ids = append(ids, pk)
			}
		}
	}

	for _, id := range ids {
		session.engine.logger.Debugf("[cache] delete cache obj: %v, %v", tableName, id)
		sid, err := id.ToString()
		if err != nil {
			return err
		}
		cacher.DelBean(tableName, sid)
	}
	session.engine.logger.Debugf("[cache] clear cache table: %v", tableName)
	cacher.ClearIds(tableName)
	return nil
}

// Delete records, bean's non-empty fields are conditions
func (session *Session) Delete(beans ...interface{}) (int64, error) {
	if session.isAutoClose {
		defer session.Close()
	}

	if session.statement.LastError != nil {
		return 0, session.statement.LastError
	}

	var (
		condSQL  string
		condArgs []interface{}
		err      error
		bean     interface{}
	)
	if len(beans) > 0 {
		bean = beans[0]
		if err = session.statement.SetRefBean(bean); err != nil {
			return 0, err
		}

		executeBeforeClosures(session, bean)

		if processor, ok := interface{}(bean).(BeforeDeleteProcessor); ok {
			processor.BeforeDelete()
		}

		condSQL, condArgs, err = session.statement.GenConds(bean)
	} else {
		condSQL, condArgs, err = session.statement.GenCondSQL(session.statement.Conds())
	}
	if err != nil {
		return 0, err
	}

	pLimitN := session.statement.LimitN
	if len(condSQL) == 0 && (pLimitN == nil || *pLimitN == 0) {
		return 0, ErrNeedDeletedCond
	}

	var tableNameNoQuote = session.statement.TableName()
	var tableName = session.engine.Quote(tableNameNoQuote)
	var table = session.statement.RefTable
	var deleteSQL string
	if len(condSQL) > 0 {
		deleteSQL = fmt.Sprintf("DELETE FROM %v WHERE %v", tableName, condSQL)
	} else {
		deleteSQL = fmt.Sprintf("DELETE FROM %v", tableName)
	}

	var orderSQL string
	if len(session.statement.OrderStr) > 0 {
		orderSQL += fmt.Sprintf(" ORDER BY %s", session.statement.OrderStr)
	}
	if pLimitN != nil && *pLimitN > 0 {
		limitNValue := *pLimitN
		orderSQL += fmt.Sprintf(" LIMIT %d", limitNValue)
	}

	if len(orderSQL) > 0 {
		switch session.engine.dialect.URI().DBType {
		case schemas.POSTGRES:
			inSQL := fmt.Sprintf("ctid IN (SELECT ctid FROM %s%s)", tableName, orderSQL)
			if len(condSQL) > 0 {
				deleteSQL += " AND " + inSQL
			} else {
				deleteSQL += " WHERE " + inSQL
			}
		case schemas.SQLITE:
			inSQL := fmt.Sprintf("rowid IN (SELECT rowid FROM %s%s)", tableName, orderSQL)
			if len(condSQL) > 0 {
				deleteSQL += " AND " + inSQL
			} else {
				deleteSQL += " WHERE " + inSQL
			}
			// TODO: how to handle delete limit on mssql?
		case schemas.MSSQL:
			return 0, ErrNotImplemented
		default:
			deleteSQL += orderSQL
		}
	}

	var realSQL string
	argsForCache := make([]interface{}, 0, len(condArgs)*2)
	if session.statement.GetUnscoped() || table == nil || table.DeletedColumn() == nil { // tag "deleted" is disabled
		realSQL = deleteSQL
		copy(argsForCache, condArgs)
		argsForCache = append(condArgs, argsForCache...)
	} else {
		// !oinume! sqlStrForCache and argsForCache is needed to behave as executing "DELETE FROM ..." for caches.
		copy(argsForCache, condArgs)
		argsForCache = append(condArgs, argsForCache...)

		deletedColumn := table.DeletedColumn()
		realSQL = fmt.Sprintf("UPDATE %v SET %v = ? WHERE %v",
			session.engine.Quote(session.statement.TableName()),
			session.engine.Quote(deletedColumn.Name),
			condSQL)

		if len(orderSQL) > 0 {
			switch session.engine.dialect.URI().DBType {
			case schemas.POSTGRES:
				inSQL := fmt.Sprintf("ctid IN (SELECT ctid FROM %s%s)", tableName, orderSQL)
				if len(condSQL) > 0 {
					realSQL += " AND " + inSQL
				} else {
					realSQL += " WHERE " + inSQL
				}
			case schemas.SQLITE:
				inSQL := fmt.Sprintf("rowid IN (SELECT rowid FROM %s%s)", tableName, orderSQL)
				if len(condSQL) > 0 {
					realSQL += " AND " + inSQL
				} else {
					realSQL += " WHERE " + inSQL
				}
				// TODO: how to handle delete limit on mssql?
			case schemas.MSSQL:
				return 0, ErrNotImplemented
			default:
				realSQL += orderSQL
			}
		}

		// !oinume! Insert nowTime to the head of session.statement.Params
		condArgs = append(condArgs, "")
		paramsLen := len(condArgs)
		copy(condArgs[1:paramsLen], condArgs[0:paramsLen-1])

		val, t, err := session.engine.nowTime(deletedColumn)
		if err != nil {
			return 0, err
		}
		condArgs[0] = val

		var colName = deletedColumn.Name
		session.afterClosures = append(session.afterClosures, func(bean interface{}) {
			col := table.GetColumn(colName)
			setColumnTime(bean, col, t)
		})
	}

	if cacher := session.engine.GetCacher(tableNameNoQuote); cacher != nil && session.statement.UseCache {
		session.cacheDelete(table, tableNameNoQuote, deleteSQL, argsForCache...)
	}

	session.statement.RefTable = table
	res, err := session.exec(realSQL, condArgs...)
	if err != nil {
		return 0, err
	}

	if bean != nil {
		// handle after delete processors
		if session.isAutoCommit {
			for _, closure := range session.afterClosures {
				closure(bean)
			}
			if processor, ok := interface{}(bean).(AfterDeleteProcessor); ok {
				processor.AfterDelete()
			}
		} else {
			lenAfterClosures := len(session.afterClosures)
			if lenAfterClosures > 0 && len(beans) > 0 {
				if value, has := session.afterDeleteBeans[beans[0]]; has && value != nil {
					*value = append(*value, session.afterClosures...)
				} else {
					afterClosures := make([]func(interface{}), lenAfterClosures)
					copy(afterClosures, session.afterClosures)
					session.afterDeleteBeans[bean] = &afterClosures
				}
			} else {
				if _, ok := interface{}(bean).(AfterDeleteProcessor); ok {
					session.afterDeleteBeans[bean] = nil
				}
			}
		}
	}
	cleanupProcessorsClosures(&session.afterClosures)
	// --

	return res.RowsAffected()
}
