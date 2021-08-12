// Copyright 2021 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"database/sql"
	"fmt"
	"math/big"
	"reflect"
	"time"

	"xorm.io/xorm/convert"
	"xorm.io/xorm/core"
	"xorm.io/xorm/dialects"
	"xorm.io/xorm/schemas"
)

// genScanResultsByBeanNullabale generates scan result
func genScanResultsByBeanNullable(bean interface{}) (interface{}, bool, error) {
	switch t := bean.(type) {
	case *interface{}:
		return t, false, nil
	case *sql.NullInt64, *sql.NullBool, *sql.NullFloat64, *sql.NullString, *sql.RawBytes:
		return t, false, nil
	case *time.Time:
		return &sql.NullString{}, true, nil
	case *sql.NullTime:
		return &sql.NullString{}, true, nil
	case *string:
		return &sql.NullString{}, true, nil
	case *int, *int8, *int16, *int32:
		return &sql.NullInt32{}, true, nil
	case *int64:
		return &sql.NullInt64{}, true, nil
	case *uint, *uint8, *uint16, *uint32:
		return &convert.NullUint32{}, true, nil
	case *uint64:
		return &convert.NullUint64{}, true, nil
	case *float32, *float64:
		return &sql.NullFloat64{}, true, nil
	case *bool:
		return &sql.NullBool{}, true, nil
	case sql.NullInt64, sql.NullBool, sql.NullFloat64, sql.NullString,
		time.Time,
		string,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64,
		bool:
		return nil, false, fmt.Errorf("unsupported scan type: %t", t)
	case convert.Conversion:
		return &sql.RawBytes{}, true, nil
	}

	tp := reflect.TypeOf(bean).Elem()
	switch tp.Kind() {
	case reflect.String:
		return &sql.NullString{}, true, nil
	case reflect.Int64:
		return &sql.NullInt64{}, true, nil
	case reflect.Int32, reflect.Int, reflect.Int16, reflect.Int8:
		return &sql.NullInt32{}, true, nil
	case reflect.Uint64:
		return &convert.NullUint64{}, true, nil
	case reflect.Uint32, reflect.Uint, reflect.Uint16, reflect.Uint8:
		return &convert.NullUint32{}, true, nil
	default:
		return nil, false, fmt.Errorf("unsupported type: %#v", bean)
	}
}

func genScanResultsByBean(bean interface{}) (interface{}, bool, error) {
	switch t := bean.(type) {
	case *interface{}:
		return t, false, nil
	case *sql.NullInt64, *sql.NullBool, *sql.NullFloat64, *sql.NullString,
		*sql.RawBytes,
		*string,
		*int, *int8, *int16, *int32, *int64,
		*uint, *uint8, *uint16, *uint32, *uint64,
		*float32, *float64,
		*bool:
		return t, false, nil
	case *time.Time, *sql.NullTime:
		return &sql.NullString{}, true, nil
	case sql.NullInt64, sql.NullBool, sql.NullFloat64, sql.NullString,
		time.Time,
		string,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		bool:
		return nil, false, fmt.Errorf("unsupported scan type: %t", t)
	case convert.Conversion:
		return &sql.RawBytes{}, true, nil
	}

	tp := reflect.TypeOf(bean).Elem()
	switch tp.Kind() {
	case reflect.String:
		return new(string), true, nil
	case reflect.Int64:
		return new(int64), true, nil
	case reflect.Int32:
		return new(int32), true, nil
	case reflect.Int:
		return new(int32), true, nil
	case reflect.Int16:
		return new(int32), true, nil
	case reflect.Int8:
		return new(int32), true, nil
	case reflect.Uint64:
		return new(uint64), true, nil
	case reflect.Uint32:
		return new(uint32), true, nil
	case reflect.Uint:
		return new(uint), true, nil
	case reflect.Uint16:
		return new(uint16), true, nil
	case reflect.Uint8:
		return new(uint8), true, nil
	case reflect.Float32:
		return new(float32), true, nil
	case reflect.Float64:
		return new(float64), true, nil
	default:
		return nil, false, fmt.Errorf("unsupported type: %#v", bean)
	}
}

func (engine *Engine) row2mapStr(rows *core.Rows, types []*sql.ColumnType, fields []string) (map[string]string, error) {
	var scanResults = make([]interface{}, len(fields))
	for i := 0; i < len(fields); i++ {
		var s sql.NullString
		scanResults[i] = &s
	}

	if err := rows.Scan(scanResults...); err != nil {
		return nil, err
	}

	result := make(map[string]string, len(fields))
	for i, key := range fields {
		s := scanResults[i].(*sql.NullString)
		if s.String == "" {
			result[key] = ""
			continue
		}

		if schemas.TIME_TYPE == engine.dialect.ColumnTypeKind(types[i].DatabaseTypeName()) {
			t, err := convert.String2Time(s.String, engine.DatabaseTZ, engine.TZLocation)
			if err != nil {
				return nil, err
			}
			result[key] = t.Format("2006-01-02 15:04:05")
		} else {
			result[key] = s.String
		}
	}
	return result, nil
}

func row2mapBytes(rows *core.Rows, types []*sql.ColumnType, fields []string) (map[string][]byte, error) {
	var scanResults = make([]interface{}, len(fields))
	for i := 0; i < len(fields); i++ {
		var s sql.NullString
		scanResults[i] = &s
	}

	if err := rows.Scan(scanResults...); err != nil {
		return nil, err
	}

	result := make(map[string][]byte, len(fields))
	for ii, key := range fields {
		s := scanResults[ii].(*sql.NullString)
		result[key] = []byte(s.String)
	}
	return result, nil
}

func (engine *Engine) scanStringInterface(rows *core.Rows, fields []string, types []*sql.ColumnType) ([]interface{}, error) {
	var scanResults = make([]interface{}, len(types))
	for i := 0; i < len(types); i++ {
		var s sql.NullString
		scanResults[i] = &s
	}

	if err := engine.scan(rows, fields, types, scanResults...); err != nil {
		return nil, err
	}
	return scanResults, nil
}

// scan is a wrap of driver.Scan but will automatically change the input values according requirements
func (engine *Engine) scan(rows *core.Rows, fields []string, types []*sql.ColumnType, vv ...interface{}) error {
	var scanResults = make([]interface{}, 0, len(types))
	var replaces = make([]bool, 0, len(types))
	var err error
	for _, v := range vv {
		var replaced bool
		var scanResult interface{}
		switch t := v.(type) {
		case *big.Float, *time.Time, *sql.NullTime:
			scanResult = &sql.NullString{}
			replaced = true
		case sql.Scanner:
			scanResult = t
		case convert.Conversion:
			scanResult = &sql.RawBytes{}
			replaced = true
		default:
			nullable, ok := types[0].Nullable()
			if !ok || nullable {
				scanResult, replaced, err = genScanResultsByBeanNullable(v)
			} else {
				scanResult, replaced, err = genScanResultsByBean(v)
			}
			if err != nil {
				return err
			}
		}

		scanResults = append(scanResults, scanResult)
		replaces = append(replaces, replaced)
	}

	if err = engine.driver.Scan(&dialects.ScanContext{
		DBLocation:   engine.DatabaseTZ,
		UserLocation: engine.TZLocation,
	}, rows, types, scanResults...); err != nil {
		return err
	}

	for i, replaced := range replaces {
		if replaced {
			if err = convert.Assign(vv[i], scanResults[i], engine.DatabaseTZ, engine.TZLocation); err != nil {
				return err
			}
		}
	}

	return nil
}

func (engine *Engine) scanInterfaces(rows *core.Rows, fields []string, types []*sql.ColumnType) ([]interface{}, error) {
	var scanResultContainers = make([]interface{}, len(types))
	for i := 0; i < len(types); i++ {
		scanResult, err := engine.driver.GenScanResult(types[i].DatabaseTypeName())
		if err != nil {
			return nil, err
		}
		scanResultContainers[i] = scanResult
	}
	if err := engine.scan(rows, fields, types, scanResultContainers...); err != nil {
		return nil, err
	}
	return scanResultContainers, nil
}

func (engine *Engine) row2sliceStr(rows *core.Rows, types []*sql.ColumnType, fields []string) ([]string, error) {
	scanResults, err := engine.scanStringInterface(rows, fields, types)
	if err != nil {
		return nil, err
	}

	var results = make([]string, 0, len(fields))
	for i := 0; i < len(fields); i++ {
		results = append(results, scanResults[i].(*sql.NullString).String)
	}
	return results, nil
}

func rows2maps(rows *core.Rows) (resultsSlice []map[string][]byte, err error) {
	fields, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		result, err := row2mapBytes(rows, types, fields)
		if err != nil {
			return nil, err
		}
		resultsSlice = append(resultsSlice, result)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return resultsSlice, nil
}

func (engine *Engine) row2mapInterface(rows *core.Rows, types []*sql.ColumnType, fields []string) (map[string]interface{}, error) {
	var resultsMap = make(map[string]interface{}, len(fields))
	var scanResultContainers = make([]interface{}, len(fields))
	for i := 0; i < len(fields); i++ {
		scanResult, err := engine.driver.GenScanResult(types[i].DatabaseTypeName())
		if err != nil {
			return nil, err
		}
		scanResultContainers[i] = scanResult
	}
	if err := engine.scan(rows, fields, types, scanResultContainers...); err != nil {
		return nil, err
	}

	for ii, key := range fields {
		res, err := convert.Interface2Interface(engine.TZLocation, scanResultContainers[ii])
		if err != nil {
			return nil, err
		}
		resultsMap[key] = res
	}
	return resultsMap, nil
}
