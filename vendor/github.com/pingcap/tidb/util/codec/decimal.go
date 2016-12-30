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

package codec

import (
	"bytes"
	"math/big"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/mysql"
)

const (
	negativeSign int64 = 8
	zeroSign     int64 = 16
	positiveSign int64 = 24
)

func codecSign(value int64) int64 {
	if value < 0 {
		return negativeSign
	}

	return positiveSign
}

func encodeExp(expValue int64, expSign int64, valSign int64) int64 {
	if expSign == negativeSign {
		expValue = -expValue
	}

	if expSign != valSign {
		expValue = ^expValue
	}

	return expValue
}

func decodeExp(expValue int64, expSign int64, valSign int64) int64 {
	if expSign != valSign {
		expValue = ^expValue
	}

	if expSign == negativeSign {
		expValue = -expValue
	}

	return expValue
}

func codecValue(value []byte, valSign int64) {
	if valSign == negativeSign {
		reverseBytes(value)
	}
}

// EncodeDecimal encodes a decimal d into a byte slice which can be sorted lexicographically later.
// EncodeDecimal guarantees that the encoded value is in ascending order for comparison.
// Decimal encoding:
// Byte -> value sign
// Byte -> exp sign
// EncodeInt -> exp value
// EncodeBytes -> abs value bytes
func EncodeDecimal(b []byte, d mysql.Decimal) []byte {
	if d.Equals(mysql.ZeroDecimal) {
		return append(b, byte(zeroSign))
	}

	v := d.BigIntValue()
	valSign := codecSign(int64(v.Sign()))

	absVal := new(big.Int)
	absVal.Abs(v)

	value := []byte(absVal.String())

	// Trim right side "0", like "12.34000" -> "12.34" or "0.1234000" -> "0.1234".
	if d.Exponent() != 0 {
		value = bytes.TrimRight(value, "0")
	}

	// Get exp and value, format is "value":"exp".
	// like "12.34" -> "0.1234":"2".
	// like "-0.01234" -> "-0.1234":"-1".
	exp := int64(0)
	div := big.NewInt(10)
	for ; ; exp++ {
		if absVal.Sign() == 0 {
			break
		}
		absVal = absVal.Div(absVal, div)
	}

	expVal := exp + int64(d.Exponent())
	expSign := codecSign(expVal)

	// For negtive exp, do bit reverse for exp.
	// For negtive decimal, do bit reverse for exp and value.
	expVal = encodeExp(expVal, expSign, valSign)
	codecValue(value, valSign)

	b = append(b, byte(valSign))
	b = append(b, byte(expSign))
	b = EncodeInt(b, expVal)
	b = EncodeBytes(b, value)
	return b
}

// DecodeDecimal decodes bytes to decimal.
// DecodeFloat decodes a float from a byte slice
// Decimal decoding:
// Byte -> value sign
// Byte -> exp sign
// DecodeInt -> exp value
// DecodeBytes -> abs value bytes
func DecodeDecimal(b []byte) ([]byte, mysql.Decimal, error) {
	var (
		r   = b
		d   mysql.Decimal
		err error
	)

	// Decode value sign.
	valSign := int64(r[0])
	r = r[1:]
	if valSign == zeroSign {
		d, err = mysql.ParseDecimal("0")
		return r, d, errors.Trace(err)
	}

	// Decode exp sign.
	expSign := int64(r[0])
	r = r[1:]

	// Decode exp value.
	expVal := int64(0)
	r, expVal, err = DecodeInt(r)
	if err != nil {
		return r, d, errors.Trace(err)
	}
	expVal = decodeExp(expVal, expSign, valSign)

	// Decode abs value bytes.
	value := []byte{}
	r, value, err = DecodeBytes(r)
	if err != nil {
		return r, d, errors.Trace(err)
	}
	codecValue(value, valSign)

	// Generate decimal string value.
	var decimalStr []byte
	if valSign == negativeSign {
		decimalStr = append(decimalStr, '-')
	}

	if expVal <= 0 {
		// Like decimal "0.1234" or "0.01234".
		decimalStr = append(decimalStr, '0')
		decimalStr = append(decimalStr, '.')
		decimalStr = append(decimalStr, bytes.Repeat([]byte{'0'}, -int(expVal))...)
		decimalStr = append(decimalStr, value...)
	} else {
		// Like decimal "12.34".
		decimalStr = append(decimalStr, value[:expVal]...)
		decimalStr = append(decimalStr, '.')
		decimalStr = append(decimalStr, value[expVal:]...)
	}

	d, err = mysql.ParseDecimal(string(decimalStr))
	return r, d, errors.Trace(err)
}
