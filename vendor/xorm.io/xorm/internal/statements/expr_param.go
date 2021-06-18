// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package statements

import (
	"fmt"
	"strings"

	"xorm.io/builder"
	"xorm.io/xorm/schemas"
)

// ErrUnsupportedExprType represents an error with unsupported express type
type ErrUnsupportedExprType struct {
	tp string
}

func (err ErrUnsupportedExprType) Error() string {
	return fmt.Sprintf("Unsupported expression type: %v", err.tp)
}

type exprParam struct {
	colName string
	arg     interface{}
}

type exprParams struct {
	ColNames []string
	Args     []interface{}
}

func (exprs *exprParams) Len() int {
	return len(exprs.ColNames)
}

func (exprs *exprParams) addParam(colName string, arg interface{}) {
	exprs.ColNames = append(exprs.ColNames, colName)
	exprs.Args = append(exprs.Args, arg)
}

func (exprs *exprParams) IsColExist(colName string) bool {
	for _, name := range exprs.ColNames {
		if strings.EqualFold(schemas.CommonQuoter.Trim(name), schemas.CommonQuoter.Trim(colName)) {
			return true
		}
	}
	return false
}

func (exprs *exprParams) getByName(colName string) (exprParam, bool) {
	for i, name := range exprs.ColNames {
		if strings.EqualFold(name, colName) {
			return exprParam{name, exprs.Args[i]}, true
		}
	}
	return exprParam{}, false
}

func (exprs *exprParams) WriteArgs(w *builder.BytesWriter) error {
	for i, expr := range exprs.Args {
		switch arg := expr.(type) {
		case *builder.Builder:
			if _, err := w.WriteString("("); err != nil {
				return err
			}
			if err := arg.WriteTo(w); err != nil {
				return err
			}
			if _, err := w.WriteString(")"); err != nil {
				return err
			}
		case string:
			if arg == "" {
				arg = "''"
			}
			if _, err := w.WriteString(fmt.Sprintf("%v", arg)); err != nil {
				return err
			}
		default:
			if _, err := w.WriteString("?"); err != nil {
				return err
			}
			w.Append(arg)
		}
		if i != len(exprs.Args)-1 {
			if _, err := w.WriteString(","); err != nil {
				return err
			}
		}
	}
	return nil
}

func (exprs *exprParams) writeNameArgs(w *builder.BytesWriter) error {
	for i, colName := range exprs.ColNames {
		if _, err := w.WriteString(colName); err != nil {
			return err
		}
		if _, err := w.WriteString("="); err != nil {
			return err
		}

		switch arg := exprs.Args[i].(type) {
		case *builder.Builder:
			if _, err := w.WriteString("("); err != nil {
				return err
			}
			if err := arg.WriteTo(w); err != nil {
				return err
			}
			if _, err := w.WriteString("("); err != nil {
				return err
			}
		default:
			w.Append(exprs.Args[i])
		}

		if i+1 != len(exprs.ColNames) {
			if _, err := w.WriteString(","); err != nil {
				return err
			}
		}
	}
	return nil
}
