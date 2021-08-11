// Copyright 2021 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package convert

import (
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
)

// AsString converts interface as string
func AsString(src interface{}) string {
	switch v := src.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case *sql.NullString:
		return v.String
	case *sql.NullInt32:
		return fmt.Sprintf("%d", v.Int32)
	case *sql.NullInt64:
		return fmt.Sprintf("%d", v.Int64)
	}
	rv := reflect.ValueOf(src)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(rv.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(rv.Uint(), 10)
	case reflect.Float64:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 64)
	case reflect.Float32:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 32)
	case reflect.Bool:
		return strconv.FormatBool(rv.Bool())
	}
	return fmt.Sprintf("%v", src)
}

// AsBytes converts interface as bytes
func AsBytes(src interface{}) ([]byte, bool) {
	switch t := src.(type) {
	case []byte:
		return t, true
	case *sql.NullString:
		if !t.Valid {
			return nil, true
		}
		return []byte(t.String), true
	case *sql.RawBytes:
		return *t, true
	}

	rv := reflect.ValueOf(src)

	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.AppendInt(nil, rv.Int(), 10), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.AppendUint(nil, rv.Uint(), 10), true
	case reflect.Float32:
		return strconv.AppendFloat(nil, rv.Float(), 'g', -1, 32), true
	case reflect.Float64:
		return strconv.AppendFloat(nil, rv.Float(), 'g', -1, 64), true
	case reflect.Bool:
		return strconv.AppendBool(nil, rv.Bool()), true
	case reflect.String:
		return []byte(rv.String()), true
	}
	return nil, false
}
