// Copyright 2016 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"database/sql"
	"strings"

	"xorm.io/xorm/core"
)

func (session *Session) queryPreprocess(sqlStr *string, paramStr ...interface{}) {
	for _, filter := range session.engine.dialect.Filters() {
		*sqlStr = filter.Do(*sqlStr)
	}

	session.lastSQL = *sqlStr
	session.lastSQLArgs = paramStr
}

func (session *Session) queryRows(sqlStr string, args ...interface{}) (*core.Rows, error) {
	defer session.resetStatement()
	if session.statement.LastError != nil {
		return nil, session.statement.LastError
	}

	session.queryPreprocess(&sqlStr, args...)

	session.lastSQL = sqlStr
	session.lastSQLArgs = args

	if session.isAutoCommit {
		var db *core.DB
		if session.sessionType == groupSession && strings.EqualFold(sqlStr[:6], "select") {
			db = session.engine.engineGroup.Slave().DB()
		} else {
			db = session.DB()
		}

		if session.prepareStmt {
			// don't clear stmt since session will cache them
			stmt, err := session.doPrepare(db, sqlStr)
			if err != nil {
				return nil, err
			}

			rows, err := stmt.QueryContext(session.ctx, args...)
			if err != nil {
				return nil, err
			}
			return rows, nil
		}

		rows, err := db.QueryContext(session.ctx, sqlStr, args...)
		if err != nil {
			return nil, err
		}
		return rows, nil
	}

	rows, err := session.tx.QueryContext(session.ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (session *Session) queryRow(sqlStr string, args ...interface{}) *core.Row {
	return core.NewRow(session.queryRows(sqlStr, args...))
}

func (session *Session) queryBytes(sqlStr string, args ...interface{}) ([]map[string][]byte, error) {
	rows, err := session.queryRows(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return rows2maps(rows)
}

func (session *Session) exec(sqlStr string, args ...interface{}) (sql.Result, error) {
	defer session.resetStatement()

	session.queryPreprocess(&sqlStr, args...)

	session.lastSQL = sqlStr
	session.lastSQLArgs = args

	if !session.isAutoCommit {
		return session.tx.ExecContext(session.ctx, sqlStr, args...)
	}

	if session.prepareStmt {
		stmt, err := session.doPrepare(session.DB(), sqlStr)
		if err != nil {
			return nil, err
		}

		res, err := stmt.ExecContext(session.ctx, args...)
		if err != nil {
			return nil, err
		}
		return res, nil
	}

	return session.DB().ExecContext(session.ctx, sqlStr, args...)
}

// Exec raw sql
func (session *Session) Exec(sqlOrArgs ...interface{}) (sql.Result, error) {
	if session.isAutoClose {
		defer session.Close()
	}

	if len(sqlOrArgs) == 0 {
		return nil, ErrUnSupportedType
	}

	sqlStr, args, err := session.statement.ConvertSQLOrArgs(sqlOrArgs...)
	if err != nil {
		return nil, err
	}

	return session.exec(sqlStr, args...)
}
