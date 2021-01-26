// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package statements

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"xorm.io/builder"
	"xorm.io/xorm/schemas"
)

func quoteNeeded(a interface{}) bool {
	switch a.(type) {
	case int, int8, int16, int32, int64:
		return false
	case uint, uint8, uint16, uint32, uint64:
		return false
	case float32, float64:
		return false
	case bool:
		return false
	case string:
		return true
	case time.Time, *time.Time:
		return true
	case builder.Builder, *builder.Builder:
		return false
	}

	t := reflect.TypeOf(a)
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return false
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return false
	case reflect.Float32, reflect.Float64:
		return false
	case reflect.Bool:
		return false
	case reflect.String:
		return true
	}

	return true
}

func convertStringSingleQuote(arg string) string {
	return "'" + strings.Replace(arg, "'", "''", -1) + "'"
}

func convertString(arg string) string {
	var buf strings.Builder
	buf.WriteRune('\'')
	for _, c := range arg {
		if c == '\\' || c == '\'' {
			buf.WriteRune('\\')
		}
		buf.WriteRune(c)
	}
	buf.WriteRune('\'')
	return buf.String()
}

func convertArg(arg interface{}, convertFunc func(string) string) string {
	if quoteNeeded(arg) {
		argv := fmt.Sprintf("%v", arg)
		return convertFunc(argv)
	}

	return fmt.Sprintf("%v", arg)
}

const insertSelectPlaceHolder = true

func (statement *Statement) WriteArg(w *builder.BytesWriter, arg interface{}) error {
	switch argv := arg.(type) {
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
		if insertSelectPlaceHolder {
			if err := w.WriteByte('?'); err != nil {
				return err
			}
			if v, ok := arg.(bool); ok && statement.dialect.URI().DBType == schemas.MSSQL {
				if v {
					w.Append(1)
				} else {
					w.Append(0)
				}
			} else {
				w.Append(arg)
			}
		} else {
			var convertFunc = convertStringSingleQuote
			if statement.dialect.URI().DBType == schemas.MYSQL {
				convertFunc = convertString
			}
			if _, err := w.WriteString(convertArg(arg, convertFunc)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (statement *Statement) WriteArgs(w *builder.BytesWriter, args []interface{}) error {
	for i, arg := range args {
		if err := statement.WriteArg(w, arg); err != nil {
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
