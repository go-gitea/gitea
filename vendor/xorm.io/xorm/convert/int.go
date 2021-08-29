// Copyright 2021 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package convert

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
	"strconv"
)

// AsInt64 converts interface as int64
func AsInt64(src interface{}) (int64, error) {
	switch v := src.(type) {
	case int:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		return int64(v), nil
	case []byte:
		return strconv.ParseInt(string(v), 10, 64)
	case string:
		return strconv.ParseInt(v, 10, 64)
	case *sql.NullString:
		return strconv.ParseInt(v.String, 10, 64)
	case *sql.NullInt32:
		return int64(v.Int32), nil
	case *sql.NullInt64:
		return int64(v.Int64), nil
	}

	rv := reflect.ValueOf(src)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(rv.Uint()), nil
	case reflect.Float64, reflect.Float32:
		return int64(rv.Float()), nil
	case reflect.String:
		return strconv.ParseInt(rv.String(), 10, 64)
	}
	return 0, fmt.Errorf("unsupported value %T as int64", src)
}

// AsUint64 converts interface as uint64
func AsUint64(src interface{}) (uint64, error) {
	switch v := src.(type) {
	case int:
		return uint64(v), nil
	case int16:
		return uint64(v), nil
	case int32:
		return uint64(v), nil
	case int8:
		return uint64(v), nil
	case int64:
		return uint64(v), nil
	case uint:
		return uint64(v), nil
	case uint8:
		return uint64(v), nil
	case uint16:
		return uint64(v), nil
	case uint32:
		return uint64(v), nil
	case uint64:
		return v, nil
	case []byte:
		return strconv.ParseUint(string(v), 10, 64)
	case string:
		return strconv.ParseUint(v, 10, 64)
	case *sql.NullString:
		return strconv.ParseUint(v.String, 10, 64)
	case *sql.NullInt32:
		return uint64(v.Int32), nil
	case *sql.NullInt64:
		return uint64(v.Int64), nil
	}

	rv := reflect.ValueOf(src)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return uint64(rv.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return uint64(rv.Uint()), nil
	case reflect.Float64, reflect.Float32:
		return uint64(rv.Float()), nil
	case reflect.String:
		return strconv.ParseUint(rv.String(), 10, 64)
	}
	return 0, fmt.Errorf("unsupported value %T as uint64", src)
}

var (
	_ sql.Scanner = &NullUint64{}
)

// NullUint64 represents an uint64 that may be null.
// NullUint64 implements the Scanner interface so
// it can be used as a scan destination, similar to NullString.
type NullUint64 struct {
	Uint64 uint64
	Valid  bool
}

// Scan implements the Scanner interface.
func (n *NullUint64) Scan(value interface{}) error {
	if value == nil {
		n.Uint64, n.Valid = 0, false
		return nil
	}
	n.Valid = true
	var err error
	n.Uint64, err = AsUint64(value)
	return err
}

// Value implements the driver Valuer interface.
func (n NullUint64) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Uint64, nil
}

var (
	_ sql.Scanner = &NullUint32{}
)

// NullUint32 represents an uint32 that may be null.
// NullUint32 implements the Scanner interface so
// it can be used as a scan destination, similar to NullString.
type NullUint32 struct {
	Uint32 uint32
	Valid  bool // Valid is true if Uint32 is not NULL
}

// Scan implements the Scanner interface.
func (n *NullUint32) Scan(value interface{}) error {
	if value == nil {
		n.Uint32, n.Valid = 0, false
		return nil
	}
	n.Valid = true
	i64, err := AsUint64(value)
	if err != nil {
		return err
	}
	n.Uint32 = uint32(i64)
	return nil
}

// Value implements the driver Valuer interface.
func (n NullUint32) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return int64(n.Uint32), nil
}
