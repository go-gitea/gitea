// Copyright 2014 The ql Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSES/QL-LICENSE file.

// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/mysql"
)

// InvConv returns a failed convertion error.
func invConv(val interface{}, tp byte) (interface{}, error) {
	return nil, errors.Errorf("cannot convert %v (type %T) to type %s", val, val, TypeStr(tp))
}

func truncateStr(str string, flen int) string {
	if flen != UnspecifiedLength && len(str) > flen {
		str = str[:flen]
	}
	return str
}

var unsignedUpperBound = map[byte]uint64{
	mysql.TypeTiny:     math.MaxUint8,
	mysql.TypeShort:    math.MaxUint16,
	mysql.TypeInt24:    mysql.MaxUint24,
	mysql.TypeLong:     math.MaxUint32,
	mysql.TypeLonglong: math.MaxUint64,
	mysql.TypeBit:      math.MaxUint64,
	mysql.TypeEnum:     math.MaxUint64,
	mysql.TypeSet:      math.MaxUint64,
}

var signedUpperBound = map[byte]int64{
	mysql.TypeTiny:     math.MaxInt8,
	mysql.TypeShort:    math.MaxInt16,
	mysql.TypeInt24:    mysql.MaxInt24,
	mysql.TypeLong:     math.MaxInt32,
	mysql.TypeLonglong: math.MaxInt64,
}

var signedLowerBound = map[byte]int64{
	mysql.TypeTiny:     math.MinInt8,
	mysql.TypeShort:    math.MinInt16,
	mysql.TypeInt24:    mysql.MinInt24,
	mysql.TypeLong:     math.MinInt32,
	mysql.TypeLonglong: math.MinInt64,
}

func convertFloatToInt(val float64, lowerBound int64, upperBound int64, tp byte) (int64, error) {
	val = RoundFloat(val)
	if val < float64(lowerBound) {
		return lowerBound, overflow(val, tp)
	}

	if val > float64(upperBound) {
		return upperBound, overflow(val, tp)
	}

	return int64(val), nil
}

func convertIntToInt(val int64, lowerBound int64, upperBound int64, tp byte) (int64, error) {
	if val < lowerBound {
		return lowerBound, overflow(val, tp)
	}

	if val > upperBound {
		return upperBound, overflow(val, tp)
	}

	return val, nil
}

func convertUintToInt(val uint64, upperBound int64, tp byte) (int64, error) {
	if val > uint64(upperBound) {
		return upperBound, overflow(val, tp)
	}

	return int64(val), nil
}

func convertToInt(val interface{}, target *FieldType) (int64, error) {
	tp := target.Tp
	lowerBound := signedLowerBound[tp]
	upperBound := signedUpperBound[tp]

	switch v := val.(type) {
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	case uint64:
		return convertUintToInt(v, upperBound, tp)
	case int:
		return convertIntToInt(int64(v), lowerBound, upperBound, tp)
	case int64:
		return convertIntToInt(int64(v), lowerBound, upperBound, tp)
	case float32:
		return convertFloatToInt(float64(v), lowerBound, upperBound, tp)
	case float64:
		return convertFloatToInt(float64(v), lowerBound, upperBound, tp)
	case string:
		fval, err := StrToFloat(v)
		if err != nil {
			return 0, errors.Trace(err)
		}
		return convertFloatToInt(fval, lowerBound, upperBound, tp)
	case []byte:
		fval, err := StrToFloat(string(v))
		if err != nil {
			return 0, errors.Trace(err)
		}
		return convertFloatToInt(fval, lowerBound, upperBound, tp)
	case mysql.Time:
		// 2011-11-10 11:11:11.999999 -> 20111110111112
		ival := v.ToNumber().Round(0).IntPart()
		return convertIntToInt(ival, lowerBound, upperBound, tp)
	case mysql.Duration:
		// 11:11:11.999999 -> 111112
		ival := v.ToNumber().Round(0).IntPart()
		return convertIntToInt(ival, lowerBound, upperBound, tp)
	case mysql.Decimal:
		fval, _ := v.Float64()
		return convertFloatToInt(fval, lowerBound, upperBound, tp)
	case mysql.Hex:
		return convertFloatToInt(v.ToNumber(), lowerBound, upperBound, tp)
	case mysql.Bit:
		return convertFloatToInt(v.ToNumber(), lowerBound, upperBound, tp)
	case mysql.Enum:
		return convertFloatToInt(v.ToNumber(), lowerBound, upperBound, tp)
	case mysql.Set:
		return convertFloatToInt(v.ToNumber(), lowerBound, upperBound, tp)
	}
	return 0, typeError(val, target)
}

func convertIntToUint(val int64, upperBound uint64, tp byte) (uint64, error) {
	if val < 0 {
		return 0, overflow(val, tp)
	}

	if uint64(val) > upperBound {
		return upperBound, overflow(val, tp)
	}

	return uint64(val), nil
}

func convertUintToUint(val uint64, upperBound uint64, tp byte) (uint64, error) {
	if val > upperBound {
		return upperBound, overflow(val, tp)
	}

	return val, nil
}

func convertFloatToUint(val float64, upperBound uint64, tp byte) (uint64, error) {
	val = RoundFloat(val)
	if val < 0 {
		return uint64(int64(val)), overflow(val, tp)
	}

	if val > float64(upperBound) {
		return upperBound, overflow(val, tp)
	}

	return uint64(val), nil
}

// typeError returns error for invalid value type.
func typeError(v interface{}, target *FieldType) error {
	return errors.Errorf("cannot use %v (type %T) in assignment to, or comparison with, column type %s)",
		v, v, target.String())
}

func isCastType(tp byte) bool {
	switch tp {
	case mysql.TypeString, mysql.TypeDuration, mysql.TypeDatetime,
		mysql.TypeDate, mysql.TypeLonglong, mysql.TypeNewDecimal:
		return true
	}
	return false
}

// Cast casts val to certain types and does not return error.
func Cast(val interface{}, target *FieldType) (interface{}, error) {
	if !isCastType(target.Tp) {
		return nil, errors.Errorf("unknown cast type - %v", target)
	}

	return Convert(val, target)
}

// Convert converts the val with type tp.
func Convert(val interface{}, target *FieldType) (v interface{}, err error) {
	d := NewDatum(val)
	ret, err := d.ConvertTo(target)
	if err != nil {
		return ret.GetValue(), errors.Trace(err)
	}
	return ret.GetValue(), nil
}

// StrToInt converts a string to an integer in best effort.
// TODO: handle overflow and add unittest.
func StrToInt(str string) (int64, error) {
	str = strings.TrimSpace(str)
	if len(str) == 0 {
		return 0, nil
	}
	negative := false
	i := 0
	if str[i] == '-' {
		negative = true
		i++
	} else if str[i] == '+' {
		i++
	}
	r := int64(0)
	for ; i < len(str); i++ {
		if !unicode.IsDigit(rune(str[i])) {
			break
		}
		r = r*10 + int64(str[i]-'0')
	}
	if negative {
		r = -r
	}
	// TODO: if i < len(str), we should return an error.
	return r, nil
}

// StrToFloat converts a string to a float64 in best effort.
func StrToFloat(str string) (float64, error) {
	str = strings.TrimSpace(str)
	if len(str) == 0 {
		return 0, nil
	}

	// MySQL uses a very loose conversation, e.g, 123.abc -> 123
	// We should do a trade off whether supporting this feature or using a strict mode.
	// Now we use a strict mode.
	return strconv.ParseFloat(str, 64)
}

// ToInt64 converts an interface to an int64.
func ToInt64(value interface{}) (int64, error) {
	return convertToInt(value, NewFieldType(mysql.TypeLonglong))
}

// ToFloat64 converts an interface to a float64.
func ToFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return float64(v), nil
	case string:
		return StrToFloat(v)
	case []byte:
		return StrToFloat(string(v))
	case mysql.Time:
		f, _ := v.ToNumber().Float64()
		return f, nil
	case mysql.Duration:
		f, _ := v.ToNumber().Float64()
		return f, nil
	case mysql.Decimal:
		vv, _ := v.Float64()
		return vv, nil
	case mysql.Hex:
		return v.ToNumber(), nil
	case mysql.Bit:
		return v.ToNumber(), nil
	case mysql.Enum:
		return v.ToNumber(), nil
	case mysql.Set:
		return v.ToNumber(), nil
	default:
		return 0, errors.Errorf("cannot convert %v(type %T) to float64", value, value)
	}
}

// ToDecimal converts an interface to a Decimal.
func ToDecimal(value interface{}) (mysql.Decimal, error) {
	switch v := value.(type) {
	case bool:
		if v {
			return mysql.ConvertToDecimal(1)
		}
		return mysql.ConvertToDecimal(0)
	case []byte:
		return mysql.ConvertToDecimal(string(v))
	case mysql.Time:
		return v.ToNumber(), nil
	case mysql.Duration:
		return v.ToNumber(), nil
	default:
		return mysql.ConvertToDecimal(value)
	}
}

// ToString converts an interface to a string.
func ToString(value interface{}) (string, error) {
	switch v := value.(type) {
	case bool:
		if v {
			return "1", nil
		}
		return "0", nil
	case int:
		return strconv.FormatInt(int64(v), 10), nil
	case int64:
		return strconv.FormatInt(int64(v), 10), nil
	case uint64:
		return strconv.FormatUint(uint64(v), 10), nil
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(float64(v), 'f', -1, 64), nil
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	case mysql.Time:
		return v.String(), nil
	case mysql.Duration:
		return v.String(), nil
	case mysql.Decimal:
		return v.String(), nil
	case mysql.Hex:
		return v.ToString(), nil
	case mysql.Bit:
		return v.ToString(), nil
	case mysql.Enum:
		return v.String(), nil
	case mysql.Set:
		return v.String(), nil
	default:
		return "", errors.Errorf("cannot convert %v(type %T) to string", value, value)
	}
}

// ToBool converts an interface to a bool.
// We will use 1 for true, and 0 for false.
func ToBool(value interface{}) (int64, error) {
	isZero := false
	switch v := value.(type) {
	case bool:
		isZero = (v == false)
	case int:
		isZero = (v == 0)
	case int64:
		isZero = (v == 0)
	case uint64:
		isZero = (v == 0)
	case float32:
		isZero = (v == 0)
	case float64:
		isZero = (v == 0)
	case string:
		if len(v) == 0 {
			isZero = true
		} else {
			n, err := StrToInt(v)
			if err != nil {
				return 0, err
			}
			isZero = (n == 0)
		}
	case []byte:
		if len(v) == 0 {
			isZero = true
		} else {
			n, err := StrToInt(string(v))
			if err != nil {
				return 0, err
			}
			isZero = (n == 0)
		}
	case mysql.Time:
		isZero = v.IsZero()
	case mysql.Duration:
		isZero = (v.Duration == 0)
	case mysql.Decimal:
		vv, _ := v.Float64()
		isZero = (vv == 0)
	case mysql.Hex:
		isZero = (v.ToNumber() == 0)
	case mysql.Bit:
		isZero = (v.ToNumber() == 0)
	case mysql.Enum:
		isZero = (v.ToNumber() == 0)
	case mysql.Set:
		isZero = (v.ToNumber() == 0)
	default:
		return 0, errors.Errorf("cannot convert %v(type %T) to bool", value, value)
	}

	if isZero {
		return 0, nil
	}

	return 1, nil
}
