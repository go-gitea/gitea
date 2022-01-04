// Copyright 2021 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package convert

import (
	"database/sql"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
)

// AsFloat64 convets interface as float64
func AsFloat64(src interface{}) (float64, error) {
	switch v := src.(type) {
	case int:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case []byte:
		return strconv.ParseFloat(string(v), 64)
	case string:
		return strconv.ParseFloat(v, 64)
	case *sql.NullString:
		return strconv.ParseFloat(v.String, 64)
	case *sql.NullInt32:
		return float64(v.Int32), nil
	case *sql.NullInt64:
		return float64(v.Int64), nil
	case *sql.NullFloat64:
		return v.Float64, nil
	}

	rv := reflect.ValueOf(src)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rv.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(rv.Uint()), nil
	case reflect.Float64, reflect.Float32:
		return float64(rv.Float()), nil
	case reflect.String:
		return strconv.ParseFloat(rv.String(), 64)
	}
	return 0, fmt.Errorf("unsupported value %T as int64", src)
}

// AsBigFloat converts interface as big.Float
func AsBigFloat(src interface{}) (*big.Float, error) {
	res := big.NewFloat(0)
	switch v := src.(type) {
	case int:
		res.SetInt64(int64(v))
		return res, nil
	case int16:
		res.SetInt64(int64(v))
		return res, nil
	case int32:
		res.SetInt64(int64(v))
		return res, nil
	case int8:
		res.SetInt64(int64(v))
		return res, nil
	case int64:
		res.SetInt64(int64(v))
		return res, nil
	case uint:
		res.SetUint64(uint64(v))
		return res, nil
	case uint8:
		res.SetUint64(uint64(v))
		return res, nil
	case uint16:
		res.SetUint64(uint64(v))
		return res, nil
	case uint32:
		res.SetUint64(uint64(v))
		return res, nil
	case uint64:
		res.SetUint64(uint64(v))
		return res, nil
	case []byte:
		res.SetString(string(v))
		return res, nil
	case string:
		res.SetString(v)
		return res, nil
	case *sql.NullString:
		if v.Valid {
			res.SetString(v.String)
			return res, nil
		}
		return nil, nil
	case *sql.NullInt32:
		if v.Valid {
			res.SetInt64(int64(v.Int32))
			return res, nil
		}
		return nil, nil
	case *sql.NullInt64:
		if v.Valid {
			res.SetInt64(int64(v.Int64))
			return res, nil
		}
		return nil, nil
	}

	rv := reflect.ValueOf(src)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		res.SetInt64(rv.Int())
		return res, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		res.SetUint64(rv.Uint())
		return res, nil
	case reflect.Float64, reflect.Float32:
		res.SetFloat64(rv.Float())
		return res, nil
	case reflect.String:
		res.SetString(rv.String())
		return res, nil
	}
	return nil, fmt.Errorf("unsupported value %T as big.Float", src)
}
