// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package statements

import (
	"fmt"
	"strings"

	"xorm.io/xorm/internal/utils"
	"xorm.io/xorm/schemas"
)

// ConvertIDSQL converts SQL with id
func (statement *Statement) ConvertIDSQL(sqlStr string) string {
	if statement.RefTable != nil {
		cols := statement.RefTable.PKColumns()
		if len(cols) == 0 {
			return ""
		}

		colstrs := statement.joinColumns(cols, false)
		sqls := utils.SplitNNoCase(sqlStr, " from ", 2)
		if len(sqls) != 2 {
			return ""
		}

		var top string
		pLimitN := statement.LimitN
		if pLimitN != nil && statement.dialect.URI().DBType == schemas.MSSQL {
			top = fmt.Sprintf("TOP %d ", *pLimitN)
		}

		newsql := fmt.Sprintf("SELECT %s%s FROM %v", top, colstrs, sqls[1])
		return newsql
	}
	return ""
}

// ConvertUpdateSQL converts update SQL
func (statement *Statement) ConvertUpdateSQL(sqlStr string) (string, string) {
	if statement.RefTable == nil || len(statement.RefTable.PrimaryKeys) != 1 {
		return "", ""
	}

	colstrs := statement.joinColumns(statement.RefTable.PKColumns(), true)
	sqls := utils.SplitNNoCase(sqlStr, "where", 2)
	if len(sqls) != 2 {
		if len(sqls) == 1 {
			return sqls[0], fmt.Sprintf("SELECT %v FROM %v",
				colstrs, statement.quote(statement.TableName()))
		}
		return "", ""
	}

	var whereStr = sqls[1]

	// TODO: for postgres only, if any other database?
	var paraStr string
	if statement.dialect.URI().DBType == schemas.POSTGRES {
		paraStr = "$"
	} else if statement.dialect.URI().DBType == schemas.MSSQL {
		paraStr = ":"
	}

	if paraStr != "" {
		if strings.Contains(sqls[1], paraStr) {
			dollers := strings.Split(sqls[1], paraStr)
			whereStr = dollers[0]
			for i, c := range dollers[1:] {
				ccs := strings.SplitN(c, " ", 2)
				whereStr += fmt.Sprintf(paraStr+"%v %v", i+1, ccs[1])
			}
		}
	}

	return sqls[0], fmt.Sprintf("SELECT %v FROM %v WHERE %v",
		colstrs, statement.quote(statement.TableName()),
		whereStr)
}
