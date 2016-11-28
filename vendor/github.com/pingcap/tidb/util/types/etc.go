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
	"fmt"
	"io"
	"strings"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/parser/opcode"
	"github.com/pingcap/tidb/terror"
	"github.com/pingcap/tidb/util/charset"
)

// IsTypeBlob returns a boolean indicating whether the tp is a blob type.
func IsTypeBlob(tp byte) bool {
	switch tp {
	case mysql.TypeTinyBlob, mysql.TypeMediumBlob, mysql.TypeBlob, mysql.TypeLongBlob:
		return true
	default:
		return false
	}
}

// IsTypeChar returns a boolean indicating
// whether the tp is the char type like a string type or a varchar type.
func IsTypeChar(tp byte) bool {
	switch tp {
	case mysql.TypeString, mysql.TypeVarchar:
		return true
	default:
		return false
	}
}

var type2Str = map[byte]string{
	mysql.TypeBit:        "bit",
	mysql.TypeBlob:       "text",
	mysql.TypeDate:       "date",
	mysql.TypeDatetime:   "datetime",
	mysql.TypeDecimal:    "decimal",
	mysql.TypeNewDecimal: "decimal",
	mysql.TypeDouble:     "double",
	mysql.TypeEnum:       "enum",
	mysql.TypeFloat:      "float",
	mysql.TypeGeometry:   "geometry",
	mysql.TypeInt24:      "mediumint",
	mysql.TypeLong:       "int",
	mysql.TypeLonglong:   "bigint",
	mysql.TypeLongBlob:   "longtext",
	mysql.TypeMediumBlob: "mediumtext",
	mysql.TypeNull:       "null",
	mysql.TypeSet:        "set",
	mysql.TypeShort:      "smallint",
	mysql.TypeString:     "char",
	mysql.TypeDuration:   "time",
	mysql.TypeTimestamp:  "timestamp",
	mysql.TypeTiny:       "tinyint",
	mysql.TypeTinyBlob:   "tinytext",
	mysql.TypeVarchar:    "varchar",
	mysql.TypeVarString:  "var_string",
	mysql.TypeYear:       "year",
}

// TypeStr converts tp to a string.
func TypeStr(tp byte) (r string) {
	return type2Str[tp]
}

// TypeToStr converts a field to a string.
// It is used for converting Text to Blob,
// or converting Char to Binary.
// Args:
//	tp: type enum
//	cs: charset
func TypeToStr(tp byte, cs string) (r string) {
	ts := type2Str[tp]
	if cs != charset.CharsetBin {
		return ts
	}
	if IsTypeBlob(tp) {
		ts = strings.Replace(ts, "text", "blob", 1)
	} else if IsTypeChar(tp) {
		ts = strings.Replace(ts, "char", "binary", 1)
	}
	return ts
}

// EOFAsNil filtrates errors,
// If err is equal to io.EOF returns nil.
func EOFAsNil(err error) error {
	if terror.ErrorEqual(err, io.EOF) {
		return nil
	}
	return errors.Trace(err)
}

// InvOp2 returns an invalid operation error.
func InvOp2(x, y interface{}, o opcode.Op) (interface{}, error) {
	return nil, errors.Errorf("Invalid operation: %v %v %v (mismatched types %T and %T)", x, o, y, x, y)
}

// UndOp returns an undefined error.
func UndOp(x interface{}, o opcode.Op) (interface{}, error) {
	return nil, errors.Errorf("Invalid operation: %v%v (operator %v not defined on %T)", o, x, o, x)
}

// Overflow returns an overflowed error.
func overflow(v interface{}, tp byte) error {
	return errors.Errorf("constant %v overflows %s", v, TypeStr(tp))
}

// TODO: collate should return errors from Compare.
func collate(x, y []interface{}) (r int) {
	nx, ny := len(x), len(y)

	switch {
	case nx == 0 && ny != 0:
		return -1
	case nx == 0 && ny == 0:
		return 0
	case nx != 0 && ny == 0:
		return 1
	}

	r = 1
	if nx > ny {
		x, y, r = y, x, -r
	}

	for i, xi := range x {
		// TODO: we may remove collate later, so here just panic error.
		c, err := Compare(xi, y[i])
		if err != nil {
			panic(fmt.Sprintf("should never happend %v", err))
		}

		if c != 0 {
			return c * r
		}
	}

	if nx == ny {
		return 0
	}

	return -r
}

// Collators maps a boolean value to a collated function.
var Collators = map[bool]func(a, b []interface{}) int{false: collateDesc, true: collate}

func collateDesc(a, b []interface{}) int {
	return -collate(a, b)
}

// IsOrderedType returns a boolean
// whether the type of y can be used by order by.
func IsOrderedType(v interface{}) (r bool) {
	switch v.(type) {
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, string, []byte,
		mysql.Decimal, mysql.Time, mysql.Duration,
		mysql.Hex, mysql.Bit, mysql.Enum, mysql.Set:
		return true
	}
	return false
}

// Clone copies an interface to another interface.
// It does a deep copy.
func Clone(from interface{}) (interface{}, error) {
	if from == nil {
		return nil, nil
	}
	switch x := from.(type) {
	case uint8, uint16, uint32, uint64, float32, float64,
		int16, int8, bool, string, int, int64, int32,
		mysql.Time, mysql.Duration, mysql.Decimal,
		mysql.Hex, mysql.Bit, mysql.Enum, mysql.Set:
		return x, nil
	case []byte:
		target := make([]byte, len(from.([]byte)))
		copy(target, from.([]byte))
		return target, nil
	case []interface{}:
		var r []interface{}
		for _, v := range from.([]interface{}) {
			vv, err := Clone(v)
			if err != nil {
				return nil, err
			}
			r = append(r, vv)
		}
		return r, nil
	default:
		return nil, errors.Errorf("Clone invalid type %T", from)
	}
}

func convergeType(a interface{}, hasDecimal, hasFloat *bool) (x interface{}) {
	x = a
	switch v := a.(type) {
	case bool:
		// treat bool as 1 and 0
		if v {
			x = int64(1)
		} else {
			x = int64(0)
		}
	case int:
		x = int64(v)
	case int8:
		x = int64(v)
	case int16:
		x = int64(v)
	case int32:
		x = int64(v)
	case int64:
		x = int64(v)
	case uint:
		x = uint64(v)
	case uint8:
		x = uint64(v)
	case uint16:
		x = uint64(v)
	case uint32:
		x = uint64(v)
	case uint64:
		x = uint64(v)
	case float32:
		x = float64(v)
		*hasFloat = true
	case float64:
		x = float64(v)
		*hasFloat = true
	case mysql.Decimal:
		x = v
		*hasDecimal = true
	}
	return
}

// Coerce changes type.
// If a or b is Decimal, changes the both to Decimal.
// Else if a or b is Float, changes the both to Float.
func Coerce(a, b interface{}) (x, y interface{}) {
	var hasDecimal bool
	var hasFloat bool
	x = convergeType(a, &hasDecimal, &hasFloat)
	y = convergeType(b, &hasDecimal, &hasFloat)
	if hasDecimal {
		d, err := mysql.ConvertToDecimal(x)
		if err == nil {
			x = d
		}
		d, err = mysql.ConvertToDecimal(y)
		if err == nil {
			y = d
		}
	} else if hasFloat {
		switch v := x.(type) {
		case int64:
			x = float64(v)
		case uint64:
			x = float64(v)
		case mysql.Hex:
			x = v.ToNumber()
		case mysql.Bit:
			x = v.ToNumber()
		case mysql.Enum:
			x = v.ToNumber()
		case mysql.Set:
			x = v.ToNumber()
		}
		switch v := y.(type) {
		case int64:
			y = float64(v)
		case uint64:
			y = float64(v)
		case mysql.Hex:
			y = v.ToNumber()
		case mysql.Bit:
			y = v.ToNumber()
		case mysql.Enum:
			y = v.ToNumber()
		case mysql.Set:
			y = v.ToNumber()
		}
	}
	return
}
