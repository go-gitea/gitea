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

package mysql

// MySQL type informations.
const (
	TypeDecimal byte = iota
	TypeTiny
	TypeShort
	TypeLong
	TypeFloat
	TypeDouble
	TypeNull
	TypeTimestamp
	TypeLonglong
	TypeInt24
	TypeDate
	TypeDuration /* Original name was TypeTime, renamed to Duration to resolve the conflict with Go type Time.*/
	TypeDatetime
	TypeYear
	TypeNewDate
	TypeVarchar
	TypeBit
)

// TypeUnspecified is an uninitialized type. TypeDecimal is not used in MySQL.
var TypeUnspecified = TypeDecimal

// MySQL type informations.
const (
	TypeNewDecimal byte = iota + 0xf6
	TypeEnum
	TypeSet
	TypeTinyBlob
	TypeMediumBlob
	TypeLongBlob
	TypeBlob
	TypeVarString
	TypeString
	TypeGeometry
)

// IsUninitializedType check if a type code is uninitialized.
// TypeDecimal is the old type code for decimal and not be used in the new mysql version.
func IsUninitializedType(tp byte) bool {
	return tp == TypeDecimal
}

// Flag informations.
const (
	NotNullFlag     = 1   /* Field can't be NULL */
	PriKeyFlag      = 2   /* Field is part of a primary key */
	UniqueKeyFlag   = 4   /* Field is part of a unique key */
	MultipleKeyFlag = 8   /* Field is part of a key */
	BlobFlag        = 16  /* Field is a blob */
	UnsignedFlag    = 32  /* Field is unsigned */
	ZerofillFlag    = 64  /* Field is zerofill */
	BinaryFlag      = 128 /* Field is binary   */

	EnumFlag           = 256    /* Field is an enum */
	AutoIncrementFlag  = 512    /* Field is an auto increment field */
	TimestampFlag      = 1024   /* Field is a timestamp */
	SetFlag            = 2048   /* Field is a set */
	NoDefaultValueFlag = 4096   /* Field doesn't have a default value */
	OnUpdateNowFlag    = 8192   /* Field is set to NOW on UPDATE */
	NumFlag            = 32768  /* Field is a num (for clients) */
	PartKeyFlag        = 16384  /* Intern: Part of some keys */
	GroupFlag          = 32768  /* Intern: Group field */
	UniqueFlag         = 65536  /* Intern: Used by sql_yacc */
	BinCmpFlag         = 131072 /* Intern: Used by sql_yacc */
)

// TypeInt24 bounds.
const (
	MaxUint24 = 1<<24 - 1
	MaxInt24  = 1<<23 - 1
	MinInt24  = -1 << 23
)

// HasNotNullFlag checks if NotNullFlag is set.
func HasNotNullFlag(flag uint) bool {
	return (flag & NotNullFlag) > 0
}

// HasNoDefaultValueFlag checks if NoDefaultValueFlag is set.
func HasNoDefaultValueFlag(flag uint) bool {
	return (flag & NoDefaultValueFlag) > 0
}

// HasAutoIncrementFlag checks if AutoIncrementFlag is set.
func HasAutoIncrementFlag(flag uint) bool {
	return (flag & AutoIncrementFlag) > 0
}

// HasUnsignedFlag checks if UnsignedFlag is set.
func HasUnsignedFlag(flag uint) bool {
	return (flag & UnsignedFlag) > 0
}

// HasZerofillFlag checks if ZerofillFlag is set.
func HasZerofillFlag(flag uint) bool {
	return (flag & ZerofillFlag) > 0
}

// HasBinaryFlag checks if BinaryFlag is set.
func HasBinaryFlag(flag uint) bool {
	return (flag & BinaryFlag) > 0
}

// HasPriKeyFlag checks if PriKeyFlag is set.
func HasPriKeyFlag(flag uint) bool {
	return (flag & PriKeyFlag) > 0
}

// HasUniKeyFlag checks if UniqueKeyFlag is set.
func HasUniKeyFlag(flag uint) bool {
	return (flag & UniqueKeyFlag) > 0
}

// HasMultipleKeyFlag checks if MultipleKeyFlag is set.
func HasMultipleKeyFlag(flag uint) bool {
	return (flag & MultipleKeyFlag) > 0
}

// HasTimestampFlag checks if HasTimestampFlag is set.
func HasTimestampFlag(flag uint) bool {
	return (flag & TimestampFlag) > 0
}

// HasOnUpdateNowFlag checks if OnUpdateNowFlag is set.
func HasOnUpdateNowFlag(flag uint) bool {
	return (flag & OnUpdateNowFlag) > 0
}
