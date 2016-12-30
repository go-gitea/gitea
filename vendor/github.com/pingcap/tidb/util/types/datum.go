// Copyright 2016 PingCAP, Inc.
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
	"time"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/util/charset"
	"github.com/pingcap/tidb/util/hack"
)

// Kind constants.
const (
	KindNull  int = 0
	KindInt64 int = iota + 1
	KindUint64
	KindFloat32
	KindFloat64
	KindString
	KindBytes
	KindMysqlBit
	KindMysqlDecimal
	KindMysqlDuration
	KindMysqlEnum
	KindMysqlHex
	KindMysqlSet
	KindMysqlTime
	KindRow
	KindInterface
	KindMinNotNull
	KindMaxValue
)

// Datum is a data box holds different kind of data.
// It has better performance and is easier to use than `interface{}`.
type Datum struct {
	k int         // datum kind.
	i int64       // i can hold int64 uint64 float64 values.
	b []byte      // b can hold string or []byte values.
	x interface{} // f hold all other types.
}

// Kind gets the kind of the datum.
func (d *Datum) Kind() int {
	return d.k
}

// GetInt64 gets int64 value.
func (d *Datum) GetInt64() int64 {
	return d.i
}

// SetInt64 sets int64 value.
func (d *Datum) SetInt64(i int64) {
	d.k = KindInt64
	d.i = i
}

// GetUint64 gets uint64 value.
func (d *Datum) GetUint64() uint64 {
	return uint64(d.i)
}

// SetUint64 sets uint64 value.
func (d *Datum) SetUint64(i uint64) {
	d.k = KindUint64
	d.i = int64(i)
}

// GetFloat64 gets float64 value.
func (d *Datum) GetFloat64() float64 {
	return math.Float64frombits(uint64(d.i))
}

// SetFloat64 sets float64 value.
func (d *Datum) SetFloat64(f float64) {
	d.k = KindFloat64
	d.i = int64(math.Float64bits(f))
}

// GetFloat32 gets float32 value.
func (d *Datum) GetFloat32() float32 {
	return float32(math.Float64frombits(uint64(d.i)))
}

// SetFloat32 sets float32 value.
func (d *Datum) SetFloat32(f float32) {
	d.k = KindFloat32
	d.i = int64(math.Float64bits(float64(f)))
}

// GetString gets string value.
func (d *Datum) GetString() string {
	return hack.String(d.b)
}

// SetString sets string value.
func (d *Datum) SetString(s string) {
	d.k = KindString
	sink(s)
	d.b = hack.Slice(s)
}

// sink prevents s from being allocated on the stack.
var sink = func(s string) {
}

// GetBytes gets bytes value.
func (d *Datum) GetBytes() []byte {
	return d.b
}

// SetBytes sets bytes value to datum.
func (d *Datum) SetBytes(b []byte) {
	d.k = KindBytes
	d.b = b
}

// SetBytesAsString sets bytes value to datum as string type.
func (d *Datum) SetBytesAsString(b []byte) {
	d.k = KindString
	d.b = b
}

// GetInterface gets interface value.
func (d *Datum) GetInterface() interface{} {
	return d.x
}

// SetInterface sets interface to datum.
func (d *Datum) SetInterface(x interface{}) {
	d.k = KindInterface
	d.x = x
}

// GetRow gets row value.
func (d *Datum) GetRow() []Datum {
	return d.x.([]Datum)
}

// SetNull sets datum to nil.
func (d *Datum) SetNull() {
	d.k = KindNull
	d.x = nil
}

// GetMysqlBit gets mysql.Bit value
func (d *Datum) GetMysqlBit() mysql.Bit {
	return d.x.(mysql.Bit)
}

// SetMysqlBit sets mysql.Bit value
func (d *Datum) SetMysqlBit(b mysql.Bit) {
	d.k = KindMysqlBit
	d.x = b
}

// GetMysqlDecimal gets mysql.Decimal value
func (d *Datum) GetMysqlDecimal() mysql.Decimal {
	return d.x.(mysql.Decimal)
}

// SetMysqlDecimal sets mysql.Decimal value
func (d *Datum) SetMysqlDecimal(b mysql.Decimal) {
	d.k = KindMysqlDecimal
	d.x = b
}

// GetMysqlDuration gets mysql.Duration value
func (d *Datum) GetMysqlDuration() mysql.Duration {
	return d.x.(mysql.Duration)
}

// SetMysqlDuration sets mysql.Duration value
func (d *Datum) SetMysqlDuration(b mysql.Duration) {
	d.k = KindMysqlDuration
	d.x = b
}

// GetMysqlEnum gets mysql.Enum value
func (d *Datum) GetMysqlEnum() mysql.Enum {
	return d.x.(mysql.Enum)
}

// SetMysqlEnum sets mysql.Enum value
func (d *Datum) SetMysqlEnum(b mysql.Enum) {
	d.k = KindMysqlEnum
	d.x = b
}

// GetMysqlHex gets mysql.Hex value
func (d *Datum) GetMysqlHex() mysql.Hex {
	return d.x.(mysql.Hex)
}

// SetMysqlHex sets mysql.Hex value
func (d *Datum) SetMysqlHex(b mysql.Hex) {
	d.k = KindMysqlHex
	d.x = b
}

// GetMysqlSet gets mysql.Set value
func (d *Datum) GetMysqlSet() mysql.Set {
	return d.x.(mysql.Set)
}

// SetMysqlSet sets mysql.Set value
func (d *Datum) SetMysqlSet(b mysql.Set) {
	d.k = KindMysqlSet
	d.x = b
}

// GetMysqlTime gets mysql.Time value
func (d *Datum) GetMysqlTime() mysql.Time {
	return d.x.(mysql.Time)
}

// SetMysqlTime sets mysql.Time value
func (d *Datum) SetMysqlTime(b mysql.Time) {
	d.k = KindMysqlTime
	d.x = b
}

// GetValue gets the value of the datum of any kind.
func (d *Datum) GetValue() interface{} {
	switch d.k {
	case KindInt64:
		return d.GetInt64()
	case KindUint64:
		return d.GetUint64()
	case KindFloat32:
		return d.GetFloat32()
	case KindFloat64:
		return d.GetFloat64()
	case KindString:
		return d.GetString()
	case KindBytes:
		return d.GetBytes()
	default:
		return d.x
	}
}

// SetValue sets any kind of value.
func (d *Datum) SetValue(val interface{}) {
	switch x := val.(type) {
	case nil:
		d.SetNull()
	case bool:
		if x {
			d.SetInt64(1)
		} else {
			d.SetInt64(0)
		}
	case int:
		d.SetInt64(int64(x))
	case int64:
		d.SetInt64(x)
	case uint64:
		d.SetUint64(x)
	case float32:
		d.SetFloat32(x)
	case float64:
		d.SetFloat64(x)
	case string:
		d.SetString(x)
	case []byte:
		d.SetBytes(x)
	case mysql.Bit:
		d.x = x
		d.k = KindMysqlBit
	case mysql.Decimal:
		d.x = x
		d.k = KindMysqlDecimal
	case mysql.Duration:
		d.x = x
		d.k = KindMysqlDuration
	case mysql.Enum:
		d.x = x
		d.k = KindMysqlEnum
	case mysql.Hex:
		d.x = x
		d.k = KindMysqlHex
	case mysql.Set:
		d.x = x
		d.k = KindMysqlSet
	case mysql.Time:
		d.x = x
		d.k = KindMysqlTime
	case []Datum:
		d.x = x
		d.k = KindRow
	default:
		d.SetInterface(x)
	}
}

// CompareDatum compares datum to another datum.
// TODO: return error properly.
func (d *Datum) CompareDatum(ad Datum) (int, error) {
	switch ad.k {
	case KindNull:
		if d.k == KindNull {
			return 0, nil
		}
		return 1, nil
	case KindMinNotNull:
		if d.k == KindNull {
			return -1, nil
		} else if d.k == KindMinNotNull {
			return 0, nil
		}
		return 1, nil
	case KindMaxValue:
		if d.k == KindMaxValue {
			return 0, nil
		}
		return -1, nil
	case KindInt64:
		return d.compareInt64(ad.GetInt64())
	case KindUint64:
		return d.compareUint64(ad.GetUint64())
	case KindFloat32, KindFloat64:
		return d.compareFloat64(ad.GetFloat64())
	case KindString:
		return d.compareString(ad.GetString())
	case KindBytes:
		return d.compareBytes(ad.GetBytes())
	case KindMysqlBit:
		return d.compareMysqlBit(ad.GetMysqlBit())
	case KindMysqlDecimal:
		return d.compareMysqlDecimal(ad.GetMysqlDecimal())
	case KindMysqlDuration:
		return d.compareMysqlDuration(ad.GetMysqlDuration())
	case KindMysqlEnum:
		return d.compareMysqlEnum(ad.GetMysqlEnum())
	case KindMysqlHex:
		return d.compareMysqlHex(ad.GetMysqlHex())
	case KindMysqlSet:
		return d.compareMysqlSet(ad.GetMysqlSet())
	case KindMysqlTime:
		return d.compareMysqlTime(ad.GetMysqlTime())
	case KindRow:
		return d.compareRow(ad.GetRow())
	default:
		return 0, nil
	}
}

func (d *Datum) compareInt64(i int64) (int, error) {
	switch d.k {
	case KindMaxValue:
		return 1, nil
	case KindInt64:
		return CompareInt64(d.i, i), nil
	case KindUint64:
		if i < 0 || d.GetUint64() > math.MaxInt64 {
			return 1, nil
		}
		return CompareInt64(d.i, i), nil
	default:
		return d.compareFloat64(float64(i))
	}
}

func (d *Datum) compareUint64(u uint64) (int, error) {
	switch d.k {
	case KindMaxValue:
		return 1, nil
	case KindInt64:
		if d.i < 0 || u > math.MaxInt64 {
			return -1, nil
		}
		return CompareInt64(d.i, int64(u)), nil
	case KindUint64:
		return CompareUint64(d.GetUint64(), u), nil
	default:
		return d.compareFloat64(float64(u))
	}
}

func (d *Datum) compareFloat64(f float64) (int, error) {
	switch d.k {
	case KindNull, KindMinNotNull:
		return -1, nil
	case KindMaxValue:
		return 1, nil
	case KindInt64:
		return CompareFloat64(float64(d.i), f), nil
	case KindUint64:
		return CompareFloat64(float64(d.GetUint64()), f), nil
	case KindFloat32, KindFloat64:
		return CompareFloat64(d.GetFloat64(), f), nil
	case KindString, KindBytes:
		fVal, err := StrToFloat(d.GetString())
		return CompareFloat64(fVal, f), err
	case KindMysqlBit:
		fVal := d.GetMysqlBit().ToNumber()
		return CompareFloat64(fVal, f), nil
	case KindMysqlDecimal:
		fVal, _ := d.GetMysqlDecimal().Float64()
		return CompareFloat64(fVal, f), nil
	case KindMysqlDuration:
		fVal := d.GetMysqlDuration().Seconds()
		return CompareFloat64(fVal, f), nil
	case KindMysqlEnum:
		fVal := d.GetMysqlEnum().ToNumber()
		return CompareFloat64(fVal, f), nil
	case KindMysqlHex:
		fVal := d.GetMysqlHex().ToNumber()
		return CompareFloat64(fVal, f), nil
	case KindMysqlSet:
		fVal := d.GetMysqlSet().ToNumber()
		return CompareFloat64(fVal, f), nil
	case KindMysqlTime:
		fVal, _ := d.GetMysqlTime().ToNumber().Float64()
		return CompareFloat64(fVal, f), nil
	default:
		return -1, nil
	}
}

func (d *Datum) compareString(s string) (int, error) {
	switch d.k {
	case KindNull, KindMinNotNull:
		return -1, nil
	case KindMaxValue:
		return 1, nil
	case KindString, KindBytes:
		return CompareString(d.GetString(), s), nil
	case KindMysqlDecimal:
		dec, err := mysql.ParseDecimal(s)
		return d.GetMysqlDecimal().Cmp(dec), err
	case KindMysqlTime:
		dt, err := mysql.ParseDatetime(s)
		return d.GetMysqlTime().Compare(dt), err
	case KindMysqlDuration:
		dur, err := mysql.ParseDuration(s, mysql.MaxFsp)
		return d.GetMysqlDuration().Compare(dur), err
	case KindMysqlBit:
		return CompareString(d.GetMysqlBit().ToString(), s), nil
	case KindMysqlHex:
		return CompareString(d.GetMysqlHex().ToString(), s), nil
	case KindMysqlSet:
		return CompareString(d.GetMysqlSet().String(), s), nil
	case KindMysqlEnum:
		return CompareString(d.GetMysqlEnum().String(), s), nil
	default:
		fVal, err := StrToFloat(s)
		if err != nil {
			return 0, err
		}
		return d.compareFloat64(fVal)
	}
}

func (d *Datum) compareBytes(b []byte) (int, error) {
	return d.compareString(hack.String(b))
}

func (d *Datum) compareMysqlBit(bit mysql.Bit) (int, error) {
	switch d.k {
	case KindString, KindBytes:
		return CompareString(d.GetString(), bit.ToString()), nil
	default:
		return d.compareFloat64(bit.ToNumber())
	}
}

func (d *Datum) compareMysqlDecimal(dec mysql.Decimal) (int, error) {
	switch d.k {
	case KindMysqlDecimal:
		return d.GetMysqlDecimal().Cmp(dec), nil
	case KindString, KindBytes:
		dDec, err := mysql.ParseDecimal(d.GetString())
		return dDec.Cmp(dec), err
	default:
		fVal, _ := dec.Float64()
		return d.compareFloat64(fVal)
	}
}

func (d *Datum) compareMysqlDuration(dur mysql.Duration) (int, error) {
	switch d.k {
	case KindMysqlDuration:
		return d.GetMysqlDuration().Compare(dur), nil
	case KindString, KindBytes:
		dDur, err := mysql.ParseDuration(d.GetString(), mysql.MaxFsp)
		return dDur.Compare(dur), err
	default:
		return d.compareFloat64(dur.Seconds())
	}
}

func (d *Datum) compareMysqlEnum(enum mysql.Enum) (int, error) {
	switch d.k {
	case KindString, KindBytes:
		return CompareString(d.GetString(), enum.String()), nil
	default:
		return d.compareFloat64(enum.ToNumber())
	}
}

func (d *Datum) compareMysqlHex(e mysql.Hex) (int, error) {
	switch d.k {
	case KindString, KindBytes:
		return CompareString(d.GetString(), e.ToString()), nil
	default:
		return d.compareFloat64(e.ToNumber())
	}
}

func (d *Datum) compareMysqlSet(set mysql.Set) (int, error) {
	switch d.k {
	case KindString, KindBytes:
		return CompareString(d.GetString(), set.String()), nil
	default:
		return d.compareFloat64(set.ToNumber())
	}
}

func (d *Datum) compareMysqlTime(time mysql.Time) (int, error) {
	switch d.k {
	case KindString, KindBytes:
		dt, err := mysql.ParseDatetime(d.GetString())
		return dt.Compare(time), err
	case KindMysqlTime:
		return d.GetMysqlTime().Compare(time), nil
	default:
		fVal, _ := time.ToNumber().Float64()
		return d.compareFloat64(fVal)
	}
}

func (d *Datum) compareRow(row []Datum) (int, error) {
	var dRow []Datum
	if d.k == KindRow {
		dRow = d.GetRow()
	} else {
		dRow = []Datum{*d}
	}
	for i := 0; i < len(row) && i < len(dRow); i++ {
		cmp, err := dRow[i].CompareDatum(row[i])
		if err != nil {
			return 0, err
		}
		if cmp != 0 {
			return cmp, nil
		}
	}
	return CompareInt64(int64(len(dRow)), int64(len(row))), nil
}

// ConvertTo converts datum to the target field type.
func (d *Datum) ConvertTo(target *FieldType) (Datum, error) {
	if d.k == KindNull {
		return Datum{}, nil
	}
	switch target.Tp { // TODO: implement mysql types convert when "CAST() AS" syntax are supported.
	case mysql.TypeTiny, mysql.TypeShort, mysql.TypeInt24, mysql.TypeLong, mysql.TypeLonglong:
		unsigned := mysql.HasUnsignedFlag(target.Flag)
		if unsigned {
			return d.convertToUint(target)
		}
		return d.convertToInt(target)
	case mysql.TypeFloat, mysql.TypeDouble:
		return d.convertToFloat(target)
	case mysql.TypeBlob, mysql.TypeTinyBlob, mysql.TypeMediumBlob, mysql.TypeLongBlob,
		mysql.TypeString, mysql.TypeVarchar, mysql.TypeVarString:
		return d.convertToString(target)
	case mysql.TypeTimestamp, mysql.TypeDatetime, mysql.TypeDate:
		return d.convertToMysqlTime(target)
	case mysql.TypeDuration:
		return d.convertToMysqlDuration(target)
	case mysql.TypeBit:
		return d.convertToMysqlBit(target)
	case mysql.TypeDecimal, mysql.TypeNewDecimal:
		return d.convertToMysqlDecimal(target)
	case mysql.TypeYear:
		return d.convertToMysqlYear(target)
	case mysql.TypeEnum:
		return d.convertToMysqlEnum(target)
	case mysql.TypeSet:
		return d.convertToMysqlSet(target)
	case mysql.TypeNull:
		return Datum{}, nil
	default:
		panic("should never happen")
	}
}

func (d *Datum) convertToFloat(target *FieldType) (Datum, error) {
	var ret Datum
	switch d.k {
	case KindNull:
		return ret, nil
	case KindInt64:
		ret.SetFloat64(float64(d.GetInt64()))
	case KindUint64:
		ret.SetFloat64(float64(d.GetUint64()))
	case KindFloat32, KindFloat64:
		ret.SetFloat64(d.GetFloat64())
	case KindString, KindBytes:
		f, err := StrToFloat(d.GetString())
		if err != nil {
			return ret, errors.Trace(err)
		}
		ret.SetFloat64(f)
	case KindMysqlTime:
		f, _ := d.GetMysqlTime().ToNumber().Float64()
		ret.SetFloat64(f)
	case KindMysqlDuration:
		f, _ := d.GetMysqlDuration().ToNumber().Float64()
		ret.SetFloat64(f)
	case KindMysqlDecimal:
		f, _ := d.GetMysqlDecimal().Float64()
		ret.SetFloat64(f)
	case KindMysqlHex:
		ret.SetFloat64(d.GetMysqlHex().ToNumber())
	case KindMysqlBit:
		ret.SetFloat64(d.GetMysqlBit().ToNumber())
	case KindMysqlSet:
		ret.SetFloat64(d.GetMysqlSet().ToNumber())
	case KindMysqlEnum:
		ret.SetFloat64(d.GetMysqlEnum().ToNumber())
	default:
		return invalidConv(d, target.Tp)
	}
	// For float and following double type, we will only truncate it for float(M, D) format.
	// If no D is set, we will handle it like origin float whether M is set or not.
	if target.Flen != UnspecifiedLength && target.Decimal != UnspecifiedLength {
		x, err := TruncateFloat(ret.GetFloat64(), target.Flen, target.Decimal)
		if err != nil {
			return ret, errors.Trace(err)
		}
		if target.Tp == mysql.TypeFloat {
			ret.SetFloat32(float32(x))
		} else {
			ret.SetFloat64(x)
		}
	}
	return ret, nil
}

func (d *Datum) convertToString(target *FieldType) (Datum, error) {
	var ret Datum
	var s string
	switch d.k {
	case KindInt64:
		s = strconv.FormatInt(d.GetInt64(), 10)
	case KindUint64:
		s = strconv.FormatUint(d.GetUint64(), 10)
	case KindFloat32:
		s = strconv.FormatFloat(d.GetFloat64(), 'f', -1, 32)
	case KindFloat64:
		s = strconv.FormatFloat(d.GetFloat64(), 'f', -1, 64)
	case KindString, KindBytes:
		s = d.GetString()
	case KindMysqlTime:
		s = d.GetMysqlTime().String()
	case KindMysqlDuration:
		s = d.GetMysqlDuration().String()
	case KindMysqlDecimal:
		s = d.GetMysqlDecimal().String()
	case KindMysqlHex:
		s = d.GetMysqlHex().ToString()
	case KindMysqlBit:
		s = d.GetMysqlBit().ToString()
	case KindMysqlEnum:
		s = d.GetMysqlEnum().String()
	case KindMysqlSet:
		s = d.GetMysqlSet().String()
	default:
		return invalidConv(d, target.Tp)
	}
	// TODO: consider target.Charset/Collate
	s = truncateStr(s, target.Flen)
	ret.SetString(s)
	if target.Charset == charset.CharsetBin {
		ret.k = KindBytes
	}
	return ret, nil
}

func (d *Datum) convertToInt(target *FieldType) (Datum, error) {
	tp := target.Tp
	lowerBound := signedLowerBound[tp]
	upperBound := signedUpperBound[tp]
	var (
		val int64
		err error
		ret Datum
	)
	switch d.k {
	case KindInt64:
		val, err = convertIntToInt(d.GetInt64(), lowerBound, upperBound, tp)
	case KindUint64:
		val, err = convertUintToInt(d.GetUint64(), upperBound, tp)
	case KindFloat32, KindFloat64:
		val, err = convertFloatToInt(d.GetFloat64(), lowerBound, upperBound, tp)
	case KindString, KindBytes:
		fval, err1 := StrToFloat(d.GetString())
		if err1 != nil {
			return ret, errors.Trace(err1)
		}
		val, err = convertFloatToInt(fval, lowerBound, upperBound, tp)
	case KindMysqlTime:
		val = d.GetMysqlTime().ToNumber().Round(0).IntPart()
		val, err = convertIntToInt(val, lowerBound, upperBound, tp)
	case KindMysqlDuration:
		val = d.GetMysqlDuration().ToNumber().Round(0).IntPart()
		val, err = convertIntToInt(val, lowerBound, upperBound, tp)
	case KindMysqlDecimal:
		fval, _ := d.GetMysqlDecimal().Float64()
		val, err = convertFloatToInt(fval, lowerBound, upperBound, tp)
	case KindMysqlHex:
		val, err = convertFloatToInt(d.GetMysqlHex().ToNumber(), lowerBound, upperBound, tp)
	case KindMysqlBit:
		val, err = convertFloatToInt(d.GetMysqlBit().ToNumber(), lowerBound, upperBound, tp)
	case KindMysqlEnum:
		val, err = convertFloatToInt(d.GetMysqlEnum().ToNumber(), lowerBound, upperBound, tp)
	case KindMysqlSet:
		val, err = convertFloatToInt(d.GetMysqlSet().ToNumber(), lowerBound, upperBound, tp)
	default:
		return invalidConv(d, target.Tp)
	}
	ret.SetInt64(val)
	if err != nil {
		return ret, errors.Trace(err)
	}
	return ret, nil
}

func (d *Datum) convertToUint(target *FieldType) (Datum, error) {
	tp := target.Tp
	upperBound := unsignedUpperBound[tp]
	var (
		val uint64
		err error
		ret Datum
	)
	switch d.k {
	case KindInt64:
		val, err = convertIntToUint(d.GetInt64(), upperBound, tp)
	case KindUint64:
		val, err = convertUintToUint(d.GetUint64(), upperBound, tp)
	case KindFloat32, KindFloat64:
		val, err = convertFloatToUint(d.GetFloat64(), upperBound, tp)
	case KindString, KindBytes:
		fval, err1 := StrToFloat(d.GetString())
		if err1 != nil {
			val, _ = convertFloatToUint(fval, upperBound, tp)
			ret.SetUint64(val)
			return ret, errors.Trace(err1)
		}
		val, err = convertFloatToUint(fval, upperBound, tp)
	case KindMysqlTime:
		ival := d.GetMysqlTime().ToNumber().Round(0).IntPart()
		val, err = convertIntToUint(ival, upperBound, tp)
	case KindMysqlDuration:
		ival := d.GetMysqlDuration().ToNumber().Round(0).IntPart()
		val, err = convertIntToUint(ival, upperBound, tp)
	case KindMysqlDecimal:
		fval, _ := d.GetMysqlDecimal().Float64()
		val, err = convertFloatToUint(fval, upperBound, tp)
	case KindMysqlHex:
		val, err = convertFloatToUint(d.GetMysqlHex().ToNumber(), upperBound, tp)
	case KindMysqlBit:
		val, err = convertFloatToUint(d.GetMysqlBit().ToNumber(), upperBound, tp)
	case KindMysqlEnum:
		val, err = convertFloatToUint(d.GetMysqlEnum().ToNumber(), upperBound, tp)
	case KindMysqlSet:
		val, err = convertFloatToUint(d.GetMysqlSet().ToNumber(), upperBound, tp)
	default:
		return invalidConv(d, target.Tp)
	}
	ret.SetUint64(val)
	if err != nil {
		return ret, errors.Trace(err)
	}
	return ret, nil
}

func (d *Datum) convertToMysqlTime(target *FieldType) (Datum, error) {
	tp := target.Tp
	fsp := mysql.DefaultFsp
	if target.Decimal != UnspecifiedLength {
		fsp = target.Decimal
	}
	var ret Datum
	switch d.k {
	case KindMysqlTime:
		t, err := d.GetMysqlTime().Convert(tp)
		if err != nil {
			ret.SetValue(t)
			return ret, errors.Trace(err)
		}
		t, err = t.RoundFrac(fsp)
		ret.SetValue(t)
		if err != nil {
			return ret, errors.Trace(err)
		}
	case KindMysqlDuration:
		t, err := d.GetMysqlDuration().ConvertToTime(tp)
		if err != nil {
			ret.SetValue(t)
			return ret, errors.Trace(err)
		}
		t, err = t.RoundFrac(fsp)
		ret.SetValue(t)
		if err != nil {
			return ret, errors.Trace(err)
		}
	case KindString, KindBytes:
		t, err := mysql.ParseTime(d.GetString(), tp, fsp)
		ret.SetValue(t)
		if err != nil {
			return ret, errors.Trace(err)
		}
	case KindInt64:
		t, err := mysql.ParseTimeFromNum(d.GetInt64(), tp, fsp)
		ret.SetValue(t)
		if err != nil {
			return ret, errors.Trace(err)
		}
	default:
		return invalidConv(d, tp)
	}
	return ret, nil
}

func (d *Datum) convertToMysqlDuration(target *FieldType) (Datum, error) {
	tp := target.Tp
	fsp := mysql.DefaultFsp
	if target.Decimal != UnspecifiedLength {
		fsp = target.Decimal
	}
	var ret Datum
	switch d.k {
	case KindMysqlTime:
		dur, err := d.GetMysqlTime().ConvertToDuration()
		if err != nil {
			ret.SetValue(dur)
			return ret, errors.Trace(err)
		}
		dur, err = dur.RoundFrac(fsp)
		ret.SetValue(dur)
		if err != nil {
			return ret, errors.Trace(err)
		}
	case KindMysqlDuration:
		dur, err := d.GetMysqlDuration().RoundFrac(fsp)
		ret.SetValue(dur)
		if err != nil {
			return ret, errors.Trace(err)
		}
	case KindString, KindBytes:
		t, err := mysql.ParseDuration(d.GetString(), fsp)
		ret.SetValue(t)
		if err != nil {
			return ret, errors.Trace(err)
		}
	default:
		return invalidConv(d, tp)
	}
	return ret, nil
}

func (d *Datum) convertToMysqlDecimal(target *FieldType) (Datum, error) {
	var ret Datum
	var dec mysql.Decimal
	switch d.k {
	case KindInt64:
		dec = mysql.NewDecimalFromInt(d.GetInt64(), 0)
	case KindUint64:
		dec = mysql.NewDecimalFromUint(d.GetUint64(), 0)
	case KindFloat32, KindFloat64:
		dec = mysql.NewDecimalFromFloat(d.GetFloat64())
	case KindString, KindBytes:
		var err error
		dec, err = mysql.ParseDecimal(d.GetString())
		if err != nil {
			return ret, errors.Trace(err)
		}
	case KindMysqlDecimal:
		dec = d.GetMysqlDecimal()
	case KindMysqlTime:
		dec = d.GetMysqlTime().ToNumber()
	case KindMysqlDuration:
		dec = d.GetMysqlDuration().ToNumber()
	case KindMysqlBit:
		dec = mysql.NewDecimalFromFloat(d.GetMysqlBit().ToNumber())
	case KindMysqlEnum:
		dec = mysql.NewDecimalFromFloat(d.GetMysqlEnum().ToNumber())
	case KindMysqlHex:
		dec = mysql.NewDecimalFromFloat(d.GetMysqlHex().ToNumber())
	case KindMysqlSet:
		dec = mysql.NewDecimalFromFloat(d.GetMysqlSet().ToNumber())
	default:
		return invalidConv(d, target.Tp)
	}
	if target.Decimal != UnspecifiedLength {
		dec = dec.Round(int32(target.Decimal))
	}
	ret.SetValue(dec)
	return ret, nil
}

func (d *Datum) convertToMysqlYear(target *FieldType) (Datum, error) {
	var (
		ret Datum
		y   int64
		err error
	)
	switch d.k {
	case KindString, KindBytes:
		y, err = StrToInt(d.GetString())
	case KindMysqlTime:
		y = int64(d.GetMysqlTime().Year())
	case KindMysqlDuration:
		y = int64(time.Now().Year())
	default:
		ret, err = d.convertToInt(NewFieldType(mysql.TypeLonglong))
		if err != nil {
			return invalidConv(d, target.Tp)
		}
		y = ret.GetInt64()
	}
	y, err = mysql.AdjustYear(y)
	if err != nil {
		return invalidConv(d, target.Tp)
	}
	ret.SetInt64(y)
	return ret, nil
}

func (d *Datum) convertToMysqlBit(target *FieldType) (Datum, error) {
	x, err := d.convertToUint(target)
	if err != nil {
		return x, errors.Trace(err)
	}
	// check bit boundary, if bit has n width, the boundary is
	// in [0, (1 << n) - 1]
	width := target.Flen
	if width == 0 || width == mysql.UnspecifiedBitWidth {
		width = mysql.MinBitWidth
	}
	maxValue := uint64(1)<<uint64(width) - 1
	val := x.GetUint64()
	if val > maxValue {
		x.SetUint64(maxValue)
		return x, overflow(val, target.Tp)
	}
	var ret Datum
	ret.SetValue(mysql.Bit{Value: val, Width: width})
	return ret, nil
}

func (d *Datum) convertToMysqlEnum(target *FieldType) (Datum, error) {
	var (
		ret Datum
		e   mysql.Enum
		err error
	)
	switch d.k {
	case KindString, KindBytes:
		e, err = mysql.ParseEnumName(target.Elems, d.GetString())
	default:
		var uintDatum Datum
		uintDatum, err = d.convertToUint(target)
		if err != nil {
			return ret, errors.Trace(err)
		}
		e, err = mysql.ParseEnumValue(target.Elems, uintDatum.GetUint64())
	}
	if err != nil {
		return invalidConv(d, target.Tp)
	}
	ret.SetValue(e)
	return ret, nil
}

func (d *Datum) convertToMysqlSet(target *FieldType) (Datum, error) {
	var (
		ret Datum
		s   mysql.Set
		err error
	)
	switch d.k {
	case KindString, KindBytes:
		s, err = mysql.ParseSetName(target.Elems, d.GetString())
	default:
		var uintDatum Datum
		uintDatum, err = d.convertToUint(target)
		if err != nil {
			return ret, errors.Trace(err)
		}
		s, err = mysql.ParseSetValue(target.Elems, uintDatum.GetUint64())
	}

	if err != nil {
		return invalidConv(d, target.Tp)
	}
	ret.SetValue(s)
	return ret, nil
}

// ToBool converts to a bool.
// We will use 1 for true, and 0 for false.
func (d *Datum) ToBool() (int64, error) {
	isZero := false
	switch d.Kind() {
	case KindInt64:
		isZero = (d.GetInt64() == 0)
	case KindUint64:
		isZero = (d.GetUint64() == 0)
	case KindFloat32:
		isZero = (d.GetFloat32() == 0)
	case KindFloat64:
		isZero = (d.GetFloat64() == 0)
	case KindString:
		s := d.GetString()
		if len(s) == 0 {
			isZero = true
		}
		n, err := StrToInt(s)
		if err != nil {
			return 0, err
		}
		isZero = (n == 0)
	case KindBytes:
		bs := d.GetBytes()
		if len(bs) == 0 {
			isZero = true
		} else {
			n, err := StrToInt(string(bs))
			if err != nil {
				return 0, err
			}
			isZero = (n == 0)
		}
	case KindMysqlTime:
		isZero = d.GetMysqlTime().IsZero()
	case KindMysqlDuration:
		isZero = (d.GetMysqlDuration().Duration == 0)
	case KindMysqlDecimal:
		v, _ := d.GetMysqlDecimal().Float64()
		isZero = (v == 0)
	case KindMysqlHex:
		isZero = (d.GetMysqlHex().ToNumber() == 0)
	case KindMysqlBit:
		isZero = (d.GetMysqlBit().ToNumber() == 0)
	case KindMysqlEnum:
		isZero = (d.GetMysqlEnum().ToNumber() == 0)
	case KindMysqlSet:
		isZero = (d.GetMysqlSet().ToNumber() == 0)
	default:
		return 0, errors.Errorf("cannot convert %v(type %T) to bool", d.GetValue(), d.GetValue())
	}
	if isZero {
		return 0, nil
	}
	return 1, nil
}

// ToInt64 converts to a int64.
func (d *Datum) ToInt64() (int64, error) {
	tp := mysql.TypeLonglong
	lowerBound := signedLowerBound[tp]
	upperBound := signedUpperBound[tp]
	switch d.Kind() {
	case KindInt64:
		return convertIntToInt(d.GetInt64(), lowerBound, upperBound, tp)
	case KindUint64:
		return convertUintToInt(d.GetUint64(), upperBound, tp)
	case KindFloat32:
		return convertFloatToInt(float64(d.GetFloat32()), lowerBound, upperBound, tp)
	case KindFloat64:
		return convertFloatToInt(d.GetFloat64(), lowerBound, upperBound, tp)
	case KindString:
		s := d.GetString()
		fval, err := StrToFloat(s)
		if err != nil {
			return 0, errors.Trace(err)
		}
		return convertFloatToInt(fval, lowerBound, upperBound, tp)
	case KindBytes:
		s := string(d.GetBytes())
		fval, err := StrToFloat(s)
		if err != nil {
			return 0, errors.Trace(err)
		}
		return convertFloatToInt(fval, lowerBound, upperBound, tp)
	case KindMysqlTime:
		// 2011-11-10 11:11:11.999999 -> 20111110111112
		ival := d.GetMysqlTime().ToNumber().Round(0).IntPart()
		return convertIntToInt(ival, lowerBound, upperBound, tp)
	case KindMysqlDuration:
		// 11:11:11.999999 -> 111112
		ival := d.GetMysqlDuration().ToNumber().Round(0).IntPart()
		return convertIntToInt(ival, lowerBound, upperBound, tp)
	case KindMysqlDecimal:
		fval, _ := d.GetMysqlDecimal().Float64()
		return convertFloatToInt(fval, lowerBound, upperBound, tp)
	case KindMysqlHex:
		fval := d.GetMysqlHex().ToNumber()
		return convertFloatToInt(fval, lowerBound, upperBound, tp)
	case KindMysqlBit:
		fval := d.GetMysqlBit().ToNumber()
		return convertFloatToInt(fval, lowerBound, upperBound, tp)
	case KindMysqlEnum:
		fval := d.GetMysqlEnum().ToNumber()
		return convertFloatToInt(fval, lowerBound, upperBound, tp)
	case KindMysqlSet:
		fval := d.GetMysqlSet().ToNumber()
		return convertFloatToInt(fval, lowerBound, upperBound, tp)
	default:
		return 0, errors.Errorf("cannot convert %v(type %T) to int64", d.GetValue(), d.GetValue())
	}
}

// ToFloat64 converts to a float64
func (d *Datum) ToFloat64() (float64, error) {
	switch d.Kind() {
	case KindInt64:
		return float64(d.GetInt64()), nil
	case KindUint64:
		return float64(d.GetUint64()), nil
	case KindFloat32:
		return float64(d.GetFloat32()), nil
	case KindFloat64:
		return d.GetFloat64(), nil
	case KindString:
		return StrToFloat(d.GetString())
	case KindBytes:
		return StrToFloat(string(d.GetBytes()))
	case KindMysqlTime:
		f, _ := d.GetMysqlTime().ToNumber().Float64()
		return f, nil
	case KindMysqlDuration:
		f, _ := d.GetMysqlDuration().ToNumber().Float64()
		return f, nil
	case KindMysqlDecimal:
		f, _ := d.GetMysqlDecimal().Float64()
		return f, nil
	case KindMysqlHex:
		return d.GetMysqlHex().ToNumber(), nil
	case KindMysqlBit:
		return d.GetMysqlBit().ToNumber(), nil
	case KindMysqlEnum:
		return d.GetMysqlEnum().ToNumber(), nil
	case KindMysqlSet:
		return d.GetMysqlSet().ToNumber(), nil
	default:
		return 0, errors.Errorf("cannot convert %v(type %T) to float64", d.GetValue(), d.GetValue())
	}
}

// ToString gets the string representation of the datum.
func (d *Datum) ToString() (string, error) {
	switch d.Kind() {
	case KindInt64:
		return strconv.FormatInt(d.GetInt64(), 10), nil
	case KindUint64:
		return strconv.FormatUint(d.GetUint64(), 10), nil
	case KindFloat32:
		return strconv.FormatFloat(float64(d.GetFloat32()), 'f', -1, 32), nil
	case KindFloat64:
		return strconv.FormatFloat(float64(d.GetFloat64()), 'f', -1, 64), nil
	case KindString:
		return d.GetString(), nil
	case KindBytes:
		return d.GetString(), nil
	case KindMysqlTime:
		return d.GetMysqlTime().String(), nil
	case KindMysqlDuration:
		return d.GetMysqlDuration().String(), nil
	case KindMysqlDecimal:
		return d.GetMysqlDecimal().String(), nil
	case KindMysqlHex:
		return d.GetMysqlHex().ToString(), nil
	case KindMysqlBit:
		return d.GetMysqlBit().ToString(), nil
	case KindMysqlEnum:
		return d.GetMysqlEnum().String(), nil
	case KindMysqlSet:
		return d.GetMysqlSet().String(), nil
	default:
		return "", errors.Errorf("cannot convert %v(type %T) to string", d.GetValue(), d.GetValue())
	}
}

func invalidConv(d *Datum, tp byte) (Datum, error) {
	return Datum{}, errors.Errorf("cannot convert %v to type %s", d, TypeStr(tp))
}

// NewDatum creates a new Datum from an interface{}.
func NewDatum(in interface{}) (d Datum) {
	switch x := in.(type) {
	case []interface{}:
		d.SetValue(MakeDatums(x...))
	default:
		d.SetValue(in)
	}
	return d
}

// MakeDatums creates datum slice from interfaces.
func MakeDatums(args ...interface{}) []Datum {
	datums := make([]Datum, len(args))
	for i, v := range args {
		datums[i] = NewDatum(v)
	}
	return datums
}

// DatumsToInterfaces converts a datum slice to interface slice.
func DatumsToInterfaces(datums []Datum) []interface{} {
	ins := make([]interface{}, len(datums))
	for i, v := range datums {
		ins[i] = v.GetValue()
	}
	return ins
}

// MinNotNullDatum returns a datum represents minimum not null value.
func MinNotNullDatum() Datum {
	return Datum{k: KindMinNotNull}
}

// MaxValueDatum returns a datum represents max value.
func MaxValueDatum() Datum {
	return Datum{k: KindMaxValue}
}
