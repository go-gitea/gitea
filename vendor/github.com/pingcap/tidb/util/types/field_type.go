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
	"strings"

	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/util/charset"
)

// UnspecifiedLength is unspecified length.
const (
	UnspecifiedLength int = -1
)

// FieldType records field type information.
type FieldType struct {
	Tp      byte
	Flag    uint
	Flen    int
	Decimal int
	Charset string
	Collate string
	// Elems is the element list for enum and set type.
	Elems []string
}

// NewFieldType returns a FieldType,
// with a type and other information about field type.
func NewFieldType(tp byte) *FieldType {
	return &FieldType{
		Tp:      tp,
		Flen:    UnspecifiedLength,
		Decimal: UnspecifiedLength,
	}
}

// CompactStr only considers Tp/CharsetBin/Flen/Deimal.
// This is used for showing column type in infoschema.
func (ft *FieldType) CompactStr() string {
	ts := TypeToStr(ft.Tp, ft.Charset)
	suffix := ""
	switch ft.Tp {
	case mysql.TypeEnum, mysql.TypeSet:
		// Format is ENUM ('e1', 'e2') or SET ('e1', 'e2')
		es := make([]string, 0, len(ft.Elems))
		for _, e := range ft.Elems {
			e = strings.Replace(e, "'", "''", -1)
			es = append(es, e)
		}
		suffix = fmt.Sprintf("('%s')", strings.Join(es, "','"))
	case mysql.TypeTimestamp, mysql.TypeDatetime, mysql.TypeDate:
		if ft.Decimal != UnspecifiedLength && ft.Decimal != 0 {
			suffix = fmt.Sprintf("(%d)", ft.Decimal)
		}
	default:
		if ft.Flen != UnspecifiedLength {
			if ft.Decimal == UnspecifiedLength {
				if ft.Tp != mysql.TypeFloat && ft.Tp != mysql.TypeDouble {
					suffix = fmt.Sprintf("(%d)", ft.Flen)
				}
			} else {
				suffix = fmt.Sprintf("(%d,%d)", ft.Flen, ft.Decimal)
			}
		} else if ft.Decimal != UnspecifiedLength {
			suffix = fmt.Sprintf("(%d)", ft.Decimal)
		}
	}
	return ts + suffix
}

// String joins the information of FieldType and
// returns a string.
func (ft *FieldType) String() string {
	strs := []string{ft.CompactStr()}
	if mysql.HasUnsignedFlag(ft.Flag) {
		strs = append(strs, "UNSIGNED")
	}
	if mysql.HasZerofillFlag(ft.Flag) {
		strs = append(strs, "ZEROFILL")
	}
	if mysql.HasBinaryFlag(ft.Flag) {
		strs = append(strs, "BINARY")
	}

	if IsTypeChar(ft.Tp) || IsTypeBlob(ft.Tp) {
		if ft.Charset != "" && ft.Charset != charset.CharsetBin {
			strs = append(strs, fmt.Sprintf("CHARACTER SET %s", ft.Charset))
		}
		if ft.Collate != "" && ft.Collate != charset.CharsetBin {
			strs = append(strs, fmt.Sprintf("COLLATE %s", ft.Collate))
		}
	}

	return strings.Join(strs, " ")
}

// DefaultTypeForValue returns the default FieldType for the value.
func DefaultTypeForValue(value interface{}) *FieldType {
	var tp *FieldType
	switch x := value.(type) {
	case nil:
		tp = NewFieldType(mysql.TypeNull)
	case bool, int64, int:
		tp = NewFieldType(mysql.TypeLonglong)
		tp.Charset = charset.CharsetBin
		tp.Collate = charset.CharsetBin
	case uint64:
		tp = NewFieldType(mysql.TypeLonglong)
		tp.Flag |= mysql.UnsignedFlag
		tp.Charset = charset.CharsetBin
		tp.Collate = charset.CharsetBin
	case string:
		tp = NewFieldType(mysql.TypeVarString)
		tp.Charset = mysql.DefaultCharset
		tp.Collate = mysql.DefaultCollationName
	case float64:
		tp = NewFieldType(mysql.TypeNewDecimal)
		tp.Charset = charset.CharsetBin
		tp.Collate = charset.CharsetBin
	case []byte:
		tp = NewFieldType(mysql.TypeBlob)
		tp.Charset = charset.CharsetBin
		tp.Collate = charset.CharsetBin
	case mysql.Bit:
		tp = NewFieldType(mysql.TypeBit)
		tp.Charset = charset.CharsetBin
		tp.Collate = charset.CharsetBin
	case mysql.Hex:
		tp = NewFieldType(mysql.TypeVarchar)
		tp.Charset = charset.CharsetBin
		tp.Collate = charset.CharsetBin
	case mysql.Time:
		tp = NewFieldType(x.Type)
		tp.Charset = charset.CharsetBin
		tp.Collate = charset.CharsetBin
	case mysql.Duration:
		tp = NewFieldType(mysql.TypeDuration)
		tp.Charset = charset.CharsetBin
		tp.Collate = charset.CharsetBin
	case mysql.Decimal:
		tp = NewFieldType(mysql.TypeNewDecimal)
		tp.Charset = charset.CharsetBin
		tp.Collate = charset.CharsetBin
	case mysql.Enum:
		tp = NewFieldType(mysql.TypeEnum)
		tp.Charset = charset.CharsetBin
		tp.Collate = charset.CharsetBin
	case mysql.Set:
		tp = NewFieldType(mysql.TypeSet)
		tp.Charset = charset.CharsetBin
		tp.Collate = charset.CharsetBin
	default:
		tp = NewFieldType(mysql.TypeDecimal)
	}
	return tp
}

// DefaultCharsetForType returns the default charset/collation for mysql type.
func DefaultCharsetForType(tp byte) (string, string) {
	switch tp {
	case mysql.TypeVarString, mysql.TypeString, mysql.TypeVarchar:
		// Default charset for string types is utf8.
		return mysql.DefaultCharset, mysql.DefaultCollationName
	}
	return charset.CharsetBin, charset.CollationBin
}

// MergeFieldType merges two MySQL type to a new type.
// This is used in hybrid field type expression.
// For example "select case c when 1 then 2 when 2 then 'tidb' from t;"
// The resule field type of the case expression is the merged type of the two when clause.
// See: https://github.com/mysql/mysql-server/blob/5.7/sql/field.cc#L1042
func MergeFieldType(a byte, b byte) byte {
	ia := getFieldTypeIndex(a)
	ib := getFieldTypeIndex(b)
	return fieldTypeMergeRules[ia][ib]
}

func getFieldTypeIndex(tp byte) int {
	itp := int(tp)
	if itp < fieldTypeTearFrom {
		return itp
	}
	return fieldTypeTearFrom + itp - fieldTypeTearTo - 1
}

const (
	fieldTypeTearFrom = int(mysql.TypeBit) + 1
	fieldTypeTearTo   = int(mysql.TypeNewDecimal) - 1
	fieldTypeNum      = fieldTypeTearFrom + (255 - fieldTypeTearTo)
)

var fieldTypeMergeRules = [fieldTypeNum][fieldTypeNum]byte{
	/* mysql.TypeDecimal -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeNewDecimal, mysql.TypeNewDecimal,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeNewDecimal, mysql.TypeNewDecimal,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeDouble, mysql.TypeDouble,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeNewDecimal, mysql.TypeVarchar,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeDecimal, mysql.TypeDecimal,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeNewDecimal, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeTiny -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeNewDecimal, mysql.TypeTiny,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeShort, mysql.TypeLong,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeFloat, mysql.TypeDouble,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeTiny, mysql.TypeVarchar,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeLonglong, mysql.TypeInt24,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeVarchar, mysql.TypeTiny,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeNewDecimal, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeShort -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeNewDecimal, mysql.TypeShort,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeShort, mysql.TypeLong,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeFloat, mysql.TypeDouble,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeShort, mysql.TypeVarchar,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeLonglong, mysql.TypeInt24,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeVarchar, mysql.TypeShort,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeNewDecimal, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeLong -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeNewDecimal, mysql.TypeLong,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeLong, mysql.TypeLong,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeDouble, mysql.TypeDouble,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeLong, mysql.TypeVarchar,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeLonglong, mysql.TypeLong,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeVarchar, mysql.TypeLong,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeNewDecimal, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeFloat -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeDouble, mysql.TypeFloat,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeFloat, mysql.TypeDouble,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeFloat, mysql.TypeDouble,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeFloat, mysql.TypeVarchar,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeFloat, mysql.TypeFloat,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeVarchar, mysql.TypeFloat,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeDouble, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeDouble -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeDouble, mysql.TypeDouble,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeDouble, mysql.TypeDouble,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeDouble, mysql.TypeDouble,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeDouble, mysql.TypeVarchar,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeDouble, mysql.TypeDouble,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeVarchar, mysql.TypeDouble,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeDouble, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeNull -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeNewDecimal, mysql.TypeTiny,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeShort, mysql.TypeLong,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeFloat, mysql.TypeDouble,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeNull, mysql.TypeTimestamp,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeLonglong, mysql.TypeLonglong,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeNewDate, mysql.TypeDuration,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeDatetime, mysql.TypeYear,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeNewDate, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeBit,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeNewDecimal, mysql.TypeEnum,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeSet, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeGeometry,
	},
	/* mysql.TypeTimestamp -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeTimestamp, mysql.TypeTimestamp,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeDatetime, mysql.TypeDatetime,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeDatetime, mysql.TypeVarchar,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeNewDate, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeLonglong -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeNewDecimal, mysql.TypeLonglong,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeLonglong, mysql.TypeLonglong,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeDouble, mysql.TypeDouble,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeLonglong, mysql.TypeVarchar,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeLonglong, mysql.TypeLong,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeVarchar, mysql.TypeLonglong,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeNewDate, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeNewDecimal, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeInt24 -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeNewDecimal, mysql.TypeInt24,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeInt24, mysql.TypeLong,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeFloat, mysql.TypeDouble,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeInt24, mysql.TypeVarchar,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeLonglong, mysql.TypeInt24,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeVarchar, mysql.TypeInt24,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeNewDate, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal    mysql.TypeEnum
		mysql.TypeNewDecimal, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeDate -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeNewDate, mysql.TypeDatetime,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeNewDate, mysql.TypeDatetime,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeDatetime, mysql.TypeVarchar,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeNewDate, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeTime -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeDuration, mysql.TypeDatetime,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeDatetime, mysql.TypeDuration,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeDatetime, mysql.TypeVarchar,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeNewDate, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeDatetime -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeDatetime, mysql.TypeDatetime,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeDatetime, mysql.TypeDatetime,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeDatetime, mysql.TypeVarchar,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeNewDate, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeYear -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeDecimal, mysql.TypeTiny,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeShort, mysql.TypeLong,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeFloat, mysql.TypeDouble,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeYear, mysql.TypeVarchar,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeLonglong, mysql.TypeInt24,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeVarchar, mysql.TypeYear,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeNewDecimal, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeNewDate -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeNewDate, mysql.TypeDatetime,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeNewDate, mysql.TypeDatetime,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeDatetime, mysql.TypeVarchar,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeNewDate, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeVarchar -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeVarchar, mysql.TypeVarchar,
	},
	/* mysql.TypeBit -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeBit, mysql.TypeVarchar,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeBit,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeNewDecimal -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeNewDecimal, mysql.TypeNewDecimal,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeNewDecimal, mysql.TypeNewDecimal,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeDouble, mysql.TypeDouble,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeNewDecimal, mysql.TypeVarchar,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeNewDecimal, mysql.TypeNewDecimal,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeVarchar, mysql.TypeNewDecimal,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeNewDecimal, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeEnum -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeEnum, mysql.TypeVarchar,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeSet -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeSet, mysql.TypeVarchar,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeVarchar,
	},
	/* mysql.TypeTinyBlob -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeTinyBlob, mysql.TypeTinyBlob,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeTinyBlob, mysql.TypeTinyBlob,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeTinyBlob, mysql.TypeTinyBlob,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeTinyBlob, mysql.TypeTinyBlob,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeTinyBlob, mysql.TypeTinyBlob,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeTinyBlob, mysql.TypeTinyBlob,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeTinyBlob, mysql.TypeTinyBlob,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeTinyBlob, mysql.TypeTinyBlob,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeTinyBlob,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeTinyBlob, mysql.TypeTinyBlob,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeTinyBlob, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeTinyBlob,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeTinyBlob, mysql.TypeTinyBlob,
	},
	/* mysql.TypeMediumBlob -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeMediumBlob, mysql.TypeMediumBlob,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeMediumBlob, mysql.TypeMediumBlob,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeMediumBlob, mysql.TypeMediumBlob,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeMediumBlob, mysql.TypeMediumBlob,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeMediumBlob, mysql.TypeMediumBlob,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeMediumBlob, mysql.TypeMediumBlob,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeMediumBlob, mysql.TypeMediumBlob,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeMediumBlob, mysql.TypeMediumBlob,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeMediumBlob,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeMediumBlob, mysql.TypeMediumBlob,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeMediumBlob, mysql.TypeMediumBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeMediumBlob, mysql.TypeMediumBlob,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeMediumBlob, mysql.TypeMediumBlob,
	},
	/* mysql.TypeLongBlob -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeLongBlob, mysql.TypeLongBlob,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeLongBlob, mysql.TypeLongBlob,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeLongBlob, mysql.TypeLongBlob,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeLongBlob, mysql.TypeLongBlob,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeLongBlob, mysql.TypeLongBlob,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeLongBlob, mysql.TypeLongBlob,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeLongBlob, mysql.TypeLongBlob,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeLongBlob, mysql.TypeLongBlob,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeLongBlob,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeLongBlob, mysql.TypeLongBlob,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeLongBlob, mysql.TypeLongBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeLongBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeLongBlob, mysql.TypeLongBlob,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeLongBlob, mysql.TypeLongBlob,
	},
	/* mysql.TypeBlob -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeBlob, mysql.TypeBlob,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeBlob, mysql.TypeBlob,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeBlob, mysql.TypeBlob,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeBlob, mysql.TypeBlob,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeBlob, mysql.TypeBlob,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeBlob, mysql.TypeBlob,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeBlob, mysql.TypeBlob,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeBlob, mysql.TypeBlob,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeBlob,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeBlob, mysql.TypeBlob,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeBlob, mysql.TypeBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeBlob,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeBlob, mysql.TypeBlob,
	},
	/* mysql.TypeVarString -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeVarchar, mysql.TypeVarchar,
	},
	/* mysql.TypeString -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeString, mysql.TypeString,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeString, mysql.TypeString,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeString, mysql.TypeString,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeString, mysql.TypeString,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeString, mysql.TypeString,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeString, mysql.TypeString,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeString, mysql.TypeString,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeString, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeString,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeString, mysql.TypeString,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeString, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeString,
	},
	/* mysql.TypeGeometry -> */
	{
		//mysql.TypeDecimal      mysql.TypeTiny
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeShort        mysql.TypeLong
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeFloat        mysql.TypeDouble
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNull         mysql.TypeTimestamp
		mysql.TypeGeometry, mysql.TypeVarchar,
		//mysql.TypeLonglong     mysql.TypeInt24
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDate         mysql.TypeTime
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeDatetime     mysql.TypeYear
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeNewDate      mysql.TypeVarchar
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeBit          <16>-<245>
		mysql.TypeVarchar,
		//mysql.TypeNewDecimal   mysql.TypeEnum
		mysql.TypeVarchar, mysql.TypeVarchar,
		//mysql.TypeSet          mysql.TypeTinyBlob
		mysql.TypeVarchar, mysql.TypeTinyBlob,
		//mysql.TypeMediumBlob  mysql.TypeLongBlob
		mysql.TypeMediumBlob, mysql.TypeLongBlob,
		//mysql.TypeBlob         mysql.TypeVarString
		mysql.TypeBlob, mysql.TypeVarchar,
		//mysql.TypeString       mysql.TypeGeometry
		mysql.TypeString, mysql.TypeGeometry,
	},
}
