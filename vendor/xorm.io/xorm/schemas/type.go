// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schemas

import (
	"reflect"
	"sort"
	"strings"
	"time"
)

// DBType represents a database type
type DBType string

// enumerates all database types
const (
	POSTGRES DBType = "postgres"
	SQLITE   DBType = "sqlite3"
	MYSQL    DBType = "mysql"
	MSSQL    DBType = "mssql"
	ORACLE   DBType = "oracle"
)

// SQLType represents SQL types
type SQLType struct {
	Name           string
	DefaultLength  int
	DefaultLength2 int
}

// enumerates all columns types
const (
	UNKNOW_TYPE = iota
	TEXT_TYPE
	BLOB_TYPE
	TIME_TYPE
	NUMERIC_TYPE
	ARRAY_TYPE
)

// IsType reutrns ture if the column type is the same as the parameter
func (s *SQLType) IsType(st int) bool {
	if t, ok := SqlTypes[s.Name]; ok && t == st {
		return true
	}
	return false
}

// IsText returns true if column is a text type
func (s *SQLType) IsText() bool {
	return s.IsType(TEXT_TYPE)
}

// IsBlob returns true if column is a binary type
func (s *SQLType) IsBlob() bool {
	return s.IsType(BLOB_TYPE)
}

// IsTime returns true if column is a time type
func (s *SQLType) IsTime() bool {
	return s.IsType(TIME_TYPE)
}

// IsNumeric returns true if column is a numeric type
func (s *SQLType) IsNumeric() bool {
	return s.IsType(NUMERIC_TYPE)
}

// IsArray returns true if column is an array type
func (s *SQLType) IsArray() bool {
	return s.IsType(ARRAY_TYPE)
}

// IsJson returns true if column is an array type
func (s *SQLType) IsJson() bool {
	return s.Name == Json || s.Name == Jsonb
}

// IsXML returns true if column is an xml type
func (s *SQLType) IsXML() bool {
	return s.Name == XML
}

// enumerates all the database column types
var (
	Bit            = "BIT"
	UnsignedBit    = "UNSIGNED BIT"
	TinyInt        = "TINYINT"
	SmallInt       = "SMALLINT"
	MediumInt      = "MEDIUMINT"
	Int            = "INT"
	UnsignedInt    = "UNSIGNED INT"
	Integer        = "INTEGER"
	BigInt         = "BIGINT"
	UnsignedBigInt = "UNSIGNED BIGINT"

	Enum = "ENUM"
	Set  = "SET"

	Char             = "CHAR"
	Varchar          = "VARCHAR"
	NChar            = "NCHAR"
	NVarchar         = "NVARCHAR"
	TinyText         = "TINYTEXT"
	Text             = "TEXT"
	NText            = "NTEXT"
	Clob             = "CLOB"
	MediumText       = "MEDIUMTEXT"
	LongText         = "LONGTEXT"
	Uuid             = "UUID"
	UniqueIdentifier = "UNIQUEIDENTIFIER"
	SysName          = "SYSNAME"

	Date          = "DATE"
	DateTime      = "DATETIME"
	SmallDateTime = "SMALLDATETIME"
	Time          = "TIME"
	TimeStamp     = "TIMESTAMP"
	TimeStampz    = "TIMESTAMPZ"
	Year          = "YEAR"

	Decimal    = "DECIMAL"
	Numeric    = "NUMERIC"
	Money      = "MONEY"
	SmallMoney = "SMALLMONEY"

	Real   = "REAL"
	Float  = "FLOAT"
	Double = "DOUBLE"

	Binary     = "BINARY"
	VarBinary  = "VARBINARY"
	TinyBlob   = "TINYBLOB"
	Blob       = "BLOB"
	MediumBlob = "MEDIUMBLOB"
	LongBlob   = "LONGBLOB"
	Bytea      = "BYTEA"

	Bool    = "BOOL"
	Boolean = "BOOLEAN"

	Serial    = "SERIAL"
	BigSerial = "BIGSERIAL"

	Json  = "JSON"
	Jsonb = "JSONB"

	XML   = "XML"
	Array = "ARRAY"

	SqlTypes = map[string]int{
		Bit:            NUMERIC_TYPE,
		UnsignedBit:    NUMERIC_TYPE,
		TinyInt:        NUMERIC_TYPE,
		SmallInt:       NUMERIC_TYPE,
		MediumInt:      NUMERIC_TYPE,
		Int:            NUMERIC_TYPE,
		UnsignedInt:    NUMERIC_TYPE,
		Integer:        NUMERIC_TYPE,
		BigInt:         NUMERIC_TYPE,
		UnsignedBigInt: NUMERIC_TYPE,

		Enum:  TEXT_TYPE,
		Set:   TEXT_TYPE,
		Json:  TEXT_TYPE,
		Jsonb: TEXT_TYPE,

		XML: TEXT_TYPE,

		Char:       TEXT_TYPE,
		NChar:      TEXT_TYPE,
		Varchar:    TEXT_TYPE,
		NVarchar:   TEXT_TYPE,
		TinyText:   TEXT_TYPE,
		Text:       TEXT_TYPE,
		NText:      TEXT_TYPE,
		MediumText: TEXT_TYPE,
		LongText:   TEXT_TYPE,
		Uuid:       TEXT_TYPE,
		Clob:       TEXT_TYPE,
		SysName:    TEXT_TYPE,

		Date:          TIME_TYPE,
		DateTime:      TIME_TYPE,
		Time:          TIME_TYPE,
		TimeStamp:     TIME_TYPE,
		TimeStampz:    TIME_TYPE,
		SmallDateTime: TIME_TYPE,
		Year:          TIME_TYPE,

		Decimal:    NUMERIC_TYPE,
		Numeric:    NUMERIC_TYPE,
		Real:       NUMERIC_TYPE,
		Float:      NUMERIC_TYPE,
		Double:     NUMERIC_TYPE,
		Money:      NUMERIC_TYPE,
		SmallMoney: NUMERIC_TYPE,

		Binary:    BLOB_TYPE,
		VarBinary: BLOB_TYPE,

		TinyBlob:         BLOB_TYPE,
		Blob:             BLOB_TYPE,
		MediumBlob:       BLOB_TYPE,
		LongBlob:         BLOB_TYPE,
		Bytea:            BLOB_TYPE,
		UniqueIdentifier: BLOB_TYPE,

		Bool: NUMERIC_TYPE,

		Serial:    NUMERIC_TYPE,
		BigSerial: NUMERIC_TYPE,

		Array: ARRAY_TYPE,
	}

	intTypes  = sort.StringSlice{"*int", "*int16", "*int32", "*int8"}
	uintTypes = sort.StringSlice{"*uint", "*uint16", "*uint32", "*uint8"}
)

// !nashtsai! treat following var as interal const values, these are used for reflect.TypeOf comparison
var (
	emptyString       string
	boolDefault       bool
	byteDefault       byte
	complex64Default  complex64
	complex128Default complex128
	float32Default    float32
	float64Default    float64
	int64Default      int64
	uint64Default     uint64
	int32Default      int32
	uint32Default     uint32
	int16Default      int16
	uint16Default     uint16
	int8Default       int8
	uint8Default      uint8
	intDefault        int
	uintDefault       uint
	timeDefault       time.Time
)

// enumerates all types
var (
	IntType   = reflect.TypeOf(intDefault)
	Int8Type  = reflect.TypeOf(int8Default)
	Int16Type = reflect.TypeOf(int16Default)
	Int32Type = reflect.TypeOf(int32Default)
	Int64Type = reflect.TypeOf(int64Default)

	UintType   = reflect.TypeOf(uintDefault)
	Uint8Type  = reflect.TypeOf(uint8Default)
	Uint16Type = reflect.TypeOf(uint16Default)
	Uint32Type = reflect.TypeOf(uint32Default)
	Uint64Type = reflect.TypeOf(uint64Default)

	Float32Type = reflect.TypeOf(float32Default)
	Float64Type = reflect.TypeOf(float64Default)

	Complex64Type  = reflect.TypeOf(complex64Default)
	Complex128Type = reflect.TypeOf(complex128Default)

	StringType = reflect.TypeOf(emptyString)
	BoolType   = reflect.TypeOf(boolDefault)
	ByteType   = reflect.TypeOf(byteDefault)
	BytesType  = reflect.SliceOf(ByteType)

	TimeType = reflect.TypeOf(timeDefault)
)

// enumerates all types
var (
	PtrIntType   = reflect.PtrTo(IntType)
	PtrInt8Type  = reflect.PtrTo(Int8Type)
	PtrInt16Type = reflect.PtrTo(Int16Type)
	PtrInt32Type = reflect.PtrTo(Int32Type)
	PtrInt64Type = reflect.PtrTo(Int64Type)

	PtrUintType   = reflect.PtrTo(UintType)
	PtrUint8Type  = reflect.PtrTo(Uint8Type)
	PtrUint16Type = reflect.PtrTo(Uint16Type)
	PtrUint32Type = reflect.PtrTo(Uint32Type)
	PtrUint64Type = reflect.PtrTo(Uint64Type)

	PtrFloat32Type = reflect.PtrTo(Float32Type)
	PtrFloat64Type = reflect.PtrTo(Float64Type)

	PtrComplex64Type  = reflect.PtrTo(Complex64Type)
	PtrComplex128Type = reflect.PtrTo(Complex128Type)

	PtrStringType = reflect.PtrTo(StringType)
	PtrBoolType   = reflect.PtrTo(BoolType)
	PtrByteType   = reflect.PtrTo(ByteType)

	PtrTimeType = reflect.PtrTo(TimeType)
)

// Type2SQLType generate SQLType acorrding Go's type
func Type2SQLType(t reflect.Type) (st SQLType) {
	switch k := t.Kind(); k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		st = SQLType{Int, 0, 0}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		st = SQLType{UnsignedInt, 0, 0}
	case reflect.Int64:
		st = SQLType{BigInt, 0, 0}
	case reflect.Uint64:
		st = SQLType{UnsignedBigInt, 0, 0}
	case reflect.Float32:
		st = SQLType{Float, 0, 0}
	case reflect.Float64:
		st = SQLType{Double, 0, 0}
	case reflect.Complex64, reflect.Complex128:
		st = SQLType{Varchar, 64, 0}
	case reflect.Array, reflect.Slice, reflect.Map:
		if t.Elem() == reflect.TypeOf(byteDefault) {
			st = SQLType{Blob, 0, 0}
		} else {
			st = SQLType{Text, 0, 0}
		}
	case reflect.Bool:
		st = SQLType{Bool, 0, 0}
	case reflect.String:
		st = SQLType{Varchar, 255, 0}
	case reflect.Struct:
		if t.ConvertibleTo(TimeType) {
			st = SQLType{DateTime, 0, 0}
		} else {
			// TODO need to handle association struct
			st = SQLType{Text, 0, 0}
		}
	case reflect.Ptr:
		st = Type2SQLType(t.Elem())
	default:
		st = SQLType{Text, 0, 0}
	}
	return
}

// SQLType2Type convert default sql type change to go types
func SQLType2Type(st SQLType) reflect.Type {
	name := strings.ToUpper(st.Name)
	switch name {
	case Bit, TinyInt, SmallInt, MediumInt, Int, Integer, Serial:
		return reflect.TypeOf(1)
	case BigInt, BigSerial:
		return reflect.TypeOf(int64(1))
	case Float, Real:
		return reflect.TypeOf(float32(1))
	case Double:
		return reflect.TypeOf(float64(1))
	case Char, NChar, Varchar, NVarchar, TinyText, Text, NText, MediumText, LongText, Enum, Set, Uuid, Clob, SysName:
		return reflect.TypeOf("")
	case TinyBlob, Blob, LongBlob, Bytea, Binary, MediumBlob, VarBinary, UniqueIdentifier:
		return reflect.TypeOf([]byte{})
	case Bool:
		return reflect.TypeOf(true)
	case DateTime, Date, Time, TimeStamp, TimeStampz, SmallDateTime, Year:
		return reflect.TypeOf(timeDefault)
	case Decimal, Numeric, Money, SmallMoney:
		return reflect.TypeOf("")
	default:
		return reflect.TypeOf("")
	}
}
