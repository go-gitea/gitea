// Copyright 2020 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package statements

import (
	"strings"

	"xorm.io/builder"
	"xorm.io/xorm/schemas"
)

func (statement *Statement) writeInsertOutput(buf *strings.Builder, table *schemas.Table) error {
	if statement.dialect.URI().DBType == schemas.MSSQL && len(table.AutoIncrement) > 0 {
		if _, err := buf.WriteString(" OUTPUT Inserted."); err != nil {
			return err
		}
		if _, err := buf.WriteString(table.AutoIncrement); err != nil {
			return err
		}
	}
	return nil
}

func (statement *Statement) GenInsertSQL(colNames []string, args []interface{}) (string, []interface{}, error) {
	var (
		table     = statement.RefTable
		tableName = statement.TableName()
		exprs     = statement.ExprColumns
		colPlaces = strings.Repeat("?, ", len(colNames))
	)
	if exprs.Len() <= 0 && len(colPlaces) > 0 {
		colPlaces = colPlaces[0 : len(colPlaces)-2]
	}

	var buf = builder.NewWriter()
	if _, err := buf.WriteString("INSERT INTO "); err != nil {
		return "", nil, err
	}

	if err := statement.dialect.Quoter().QuoteTo(buf.Builder, tableName); err != nil {
		return "", nil, err
	}

	if len(colPlaces) <= 0 {
		if statement.dialect.URI().DBType == schemas.MYSQL {
			if _, err := buf.WriteString(" VALUES ()"); err != nil {
				return "", nil, err
			}
		} else {
			if err := statement.writeInsertOutput(buf.Builder, table); err != nil {
				return "", nil, err
			}
			if _, err := buf.WriteString(" DEFAULT VALUES"); err != nil {
				return "", nil, err
			}
		}
	} else {
		if _, err := buf.WriteString(" ("); err != nil {
			return "", nil, err
		}

		if err := statement.dialect.Quoter().JoinWrite(buf.Builder, append(colNames, exprs.ColNames...), ","); err != nil {
			return "", nil, err
		}

		if statement.Conds().IsValid() {
			if _, err := buf.WriteString(")"); err != nil {
				return "", nil, err
			}
			if err := statement.writeInsertOutput(buf.Builder, table); err != nil {
				return "", nil, err
			}
			if _, err := buf.WriteString(" SELECT "); err != nil {
				return "", nil, err
			}

			if err := statement.WriteArgs(buf, args); err != nil {
				return "", nil, err
			}

			if len(exprs.Args) > 0 {
				if _, err := buf.WriteString(","); err != nil {
					return "", nil, err
				}
			}
			if err := exprs.WriteArgs(buf); err != nil {
				return "", nil, err
			}

			if _, err := buf.WriteString(" FROM "); err != nil {
				return "", nil, err
			}

			if err := statement.dialect.Quoter().QuoteTo(buf.Builder, tableName); err != nil {
				return "", nil, err
			}

			if _, err := buf.WriteString(" WHERE "); err != nil {
				return "", nil, err
			}

			if err := statement.Conds().WriteTo(buf); err != nil {
				return "", nil, err
			}
		} else {
			buf.Append(args...)

			if _, err := buf.WriteString(")"); err != nil {
				return "", nil, err
			}
			if err := statement.writeInsertOutput(buf.Builder, table); err != nil {
				return "", nil, err
			}
			if _, err := buf.WriteString(" VALUES ("); err != nil {
				return "", nil, err
			}
			if _, err := buf.WriteString(colPlaces); err != nil {
				return "", nil, err
			}

			if err := exprs.WriteArgs(buf); err != nil {
				return "", nil, err
			}

			if _, err := buf.WriteString(")"); err != nil {
				return "", nil, err
			}
		}
	}

	if len(table.AutoIncrement) > 0 && statement.dialect.URI().DBType == schemas.POSTGRES {
		if _, err := buf.WriteString(" RETURNING "); err != nil {
			return "", nil, err
		}
		if err := statement.dialect.Quoter().QuoteTo(buf.Builder, table.AutoIncrement); err != nil {
			return "", nil, err
		}
	}

	return buf.String(), buf.Args(), nil
}
