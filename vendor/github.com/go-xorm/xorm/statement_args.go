// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"fmt"

	"xorm.io/builder"
	"xorm.io/core"
)

func (statement *Statement) writeArg(w *builder.BytesWriter, arg interface{}) error {
	switch argv := arg.(type) {
	case string:
		if _, err := w.WriteString("'" + argv + "'"); err != nil {
			return err
		}
	case bool:
		if statement.Engine.dialect.DBType() == core.MSSQL {
			if argv {
				if _, err := w.WriteString("1"); err != nil {
					return err
				}
			} else {
				if _, err := w.WriteString("0"); err != nil {
					return err
				}
			}
		} else {
			if argv {
				if _, err := w.WriteString("true"); err != nil {
					return err
				}
			} else {
				if _, err := w.WriteString("false"); err != nil {
					return err
				}
			}
		}
	case *builder.Builder:
		if _, err := w.WriteString("("); err != nil {
			return err
		}
		if err := argv.WriteTo(w); err != nil {
			return err
		}
		if _, err := w.WriteString(")"); err != nil {
			return err
		}
	default:
		if _, err := w.WriteString(fmt.Sprintf("%v", argv)); err != nil {
			return err
		}
	}
	return nil
}

func (statement *Statement) writeArgs(w *builder.BytesWriter, args []interface{}) error {
	for i, arg := range args {
		if err := statement.writeArg(w, arg); err != nil {
			return err
		}

		if i+1 != len(args) {
			if _, err := w.WriteString(","); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeStrings(w *builder.BytesWriter, cols []string, leftQuote, rightQuote string) error {
	for i, colName := range cols {
		if len(leftQuote) > 0 && colName[0] != '`' {
			if _, err := w.WriteString(leftQuote); err != nil {
				return err
			}
		}
		if _, err := w.WriteString(colName); err != nil {
			return err
		}
		if len(rightQuote) > 0 && colName[len(colName)-1] != '`' {
			if _, err := w.WriteString(rightQuote); err != nil {
				return err
			}
		}
		if i+1 != len(cols) {
			if _, err := w.WriteString(","); err != nil {
				return err
			}
		}
	}
	return nil
}
