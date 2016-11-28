// The MIT License (MIT)

// Copyright (c) 2015 Spring, Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// - Based on https://github.com/oguzbilgic/fpd, which has the following license:
// """
// The MIT License (MIT)

// Copyright (c) 2013 Oguz Bilgic

// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
// """

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

// Decimal implements an arbitrary precision fixed-point decimal.
//
// To use as part of a struct:
//
//     type Struct struct {
//         Number Decimal
//     }
//
// The zero-value of a Decimal is 0, as you would expect.
//
// The best way to create a new Decimal is to use decimal.NewFromString, ex:
//
//     n, err := decimal.NewFromString("-123.4567")
//     n.String() // output: "-123.4567"
//
// NOTE: this can "only" represent numbers with a maximum of 2^31 digits
// after the decimal point.

import (
	"database/sql/driver"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// DivisionPrecision is the number of decimal places in the result when it
// doesn't divide exactly.
//
// Example:
//
//     d1 := decimal.NewFromFloat(2).Div(decimal.NewFromFloat(3)
//     d1.String() // output: "0.6667"
//     d2 := decimal.NewFromFloat(2).Div(decimal.NewFromFloat(30000)
//     d2.String() // output: "0.0001"
//     d3 := decimal.NewFromFloat(20000).Div(decimal.NewFromFloat(3)
//     d3.String() // output: "6666.6666666666666667"
//     decimal.DivisionPrecision = 3
//     d4 := decimal.NewFromFloat(2).Div(decimal.NewFromFloat(3)
//     d4.String() // output: "0.6667"
//
const (
	MaxFractionDigits    = 30
	DivIncreasePrecision = 4
)

// ZeroDecimal is zero constant, to make computations faster.
var ZeroDecimal = NewDecimalFromInt(0, 1)

var zeroInt = big.NewInt(0)
var oneInt = big.NewInt(1)
var fiveInt = big.NewInt(5)
var tenInt = big.NewInt(10)

// Decimal represents a fixed-point decimal. It is immutable.
// number = value * 10 ^ exp
type Decimal struct {
	value *big.Int

	// this must be an int32, because we cast it to float64 during
	// calculations. If exp is 64 bit, we might lose precision.
	// If we cared about being able to represent every possible decimal, we
	// could make exp a *big.Int but it would hurt performance and numbers
	// like that are unrealistic.
	exp        int32
	fracDigits int32 // Number of fractional digits for string result.
}

// ConvertToDecimal converts interface to decimal.
func ConvertToDecimal(value interface{}) (Decimal, error) {
	switch v := value.(type) {
	case int8:
		return NewDecimalFromInt(int64(v), 0), nil
	case int16:
		return NewDecimalFromInt(int64(v), 0), nil
	case int32:
		return NewDecimalFromInt(int64(v), 0), nil
	case int64:
		return NewDecimalFromInt(int64(v), 0), nil
	case int:
		return NewDecimalFromInt(int64(v), 0), nil
	case uint8:
		return NewDecimalFromUint(uint64(v), 0), nil
	case uint16:
		return NewDecimalFromUint(uint64(v), 0), nil
	case uint32:
		return NewDecimalFromUint(uint64(v), 0), nil
	case uint64:
		return NewDecimalFromUint(uint64(v), 0), nil
	case uint:
		return NewDecimalFromUint(uint64(v), 0), nil
	case float32:
		return NewDecimalFromFloat(float64(v)), nil
	case float64:
		return NewDecimalFromFloat(float64(v)), nil
	case string:
		return ParseDecimal(v)
	case Decimal:
		return v, nil
	case Hex:
		return NewDecimalFromInt(int64(v.Value), 0), nil
	case Bit:
		return NewDecimalFromUint(uint64(v.Value), 0), nil
	case Enum:
		return NewDecimalFromUint(uint64(v.Value), 0), nil
	case Set:
		return NewDecimalFromUint(uint64(v.Value), 0), nil
	default:
		return Decimal{}, fmt.Errorf("can't convert %v to decimal", value)
	}
}

// NewDecimalFromInt returns a new fixed-point decimal, value * 10 ^ exp.
func NewDecimalFromInt(value int64, exp int32) Decimal {
	return Decimal{
		value:      big.NewInt(value),
		exp:        exp,
		fracDigits: fracDigitsDefault(exp),
	}
}

// NewDecimalFromUint returns a new fixed-point decimal, value * 10 ^ exp.
func NewDecimalFromUint(value uint64, exp int32) Decimal {
	return Decimal{
		value:      big.NewInt(0).SetUint64(value),
		exp:        exp,
		fracDigits: fracDigitsDefault(exp),
	}
}

// ParseDecimal returns a new Decimal from a string representation.
//
// Example:
//
//     d, err := ParseDecimal("-123.45")
//     d2, err := ParseDecimal(".0001")
//
func ParseDecimal(value string) (Decimal, error) {
	var intString string
	var exp = int32(0)

	n := strings.IndexAny(value, "eE")
	if n > 0 {
		// It is scientific notation, like 3.14e10
		expInt, err := strconv.Atoi(value[n+1:])
		if err != nil {
			return Decimal{}, fmt.Errorf("can't convert %s to decimal, incorrect exponent", value)
		}
		value = value[0:n]
		exp = int32(expInt)
	}

	parts := strings.Split(value, ".")
	if len(parts) == 1 {
		// There is no decimal point, we can just parse the original string as
		// an int.
		intString = value
	} else if len(parts) == 2 {
		intString = parts[0] + parts[1]
		expInt := -len(parts[1])
		exp += int32(expInt)
	} else {
		return Decimal{}, fmt.Errorf("can't convert %s to decimal: too many .s", value)
	}

	dValue := new(big.Int)
	_, ok := dValue.SetString(intString, 10)
	if !ok {
		return Decimal{}, fmt.Errorf("can't convert %s to decimal", value)
	}

	val := Decimal{
		value:      dValue,
		exp:        exp,
		fracDigits: fracDigitsDefault(exp),
	}
	if exp < -MaxFractionDigits {
		val = val.rescale(-MaxFractionDigits)
	}
	return val, nil
}

// NewDecimalFromFloat converts a float64 to Decimal.
//
// Example:
//
//     NewDecimalFromFloat(123.45678901234567).String() // output: "123.4567890123456"
//     NewDecimalFromFloat(.00000000000000001).String() // output: "0.00000000000000001"
//
// NOTE: this will panic on NaN, +/-inf.
func NewDecimalFromFloat(value float64) Decimal {
	floor := math.Floor(value)

	// fast path, where float is an int.
	if floor == value && !math.IsInf(value, 0) {
		return NewDecimalFromInt(int64(value), 0)
	}

	str := strconv.FormatFloat(value, 'f', -1, 64)
	dec, err := ParseDecimal(str)
	if err != nil {
		panic(err)
	}
	return dec
}

// NewDecimalFromFloatWithExponent converts a float64 to Decimal, with an arbitrary
// number of fractional digits.
//
// Example:
//
//     NewDecimalFromFloatWithExponent(123.456, -2).String() // output: "123.46"
//
func NewDecimalFromFloatWithExponent(value float64, exp int32) Decimal {
	mul := math.Pow(10, -float64(exp))
	floatValue := value * mul
	if math.IsNaN(floatValue) || math.IsInf(floatValue, 0) {
		panic(fmt.Sprintf("Cannot create a Decimal from %v", floatValue))
	}
	dValue := big.NewInt(round(floatValue))

	return Decimal{
		value:      dValue,
		exp:        exp,
		fracDigits: fracDigitsDefault(exp),
	}
}

// rescale returns a rescaled version of the decimal. Returned
// decimal may be less precise if the given exponent is bigger
// than the initial exponent of the Decimal.
// NOTE: this will truncate, NOT round
//
// Example:
//
// 	d := New(12345, -4)
//	d2 := d.rescale(-1)
//	d3 := d2.rescale(-4)
//	println(d1)
//	println(d2)
//	println(d3)
//
// Output:
//
//	1.2345
//	1.2
//	1.2000
//
func (d Decimal) rescale(exp int32) Decimal {
	d.ensureInitialized()
	if exp < -MaxFractionDigits-1 {
		// Limit the number of digits but we can not call Round here because it is called by Round.
		// Limit it to MaxFractionDigits + 1 to make sure the final result is correct.
		exp = -MaxFractionDigits - 1
	}
	// Must convert exps to float64 before - to prevent overflow.
	diff := math.Abs(float64(exp) - float64(d.exp))
	value := new(big.Int).Set(d.value)

	expScale := new(big.Int).Exp(tenInt, big.NewInt(int64(diff)), nil)
	if exp > d.exp {
		value = value.Quo(value, expScale)
	} else if exp < d.exp {
		value = value.Mul(value, expScale)
	}
	return Decimal{
		value:      value,
		exp:        exp,
		fracDigits: d.fracDigits,
	}
}

// Abs returns the absolute value of the decimal.
func (d Decimal) Abs() Decimal {
	d.ensureInitialized()
	d2Value := new(big.Int).Abs(d.value)
	return Decimal{
		value:      d2Value,
		exp:        d.exp,
		fracDigits: d.fracDigits,
	}
}

// Add returns d + d2.
func (d Decimal) Add(d2 Decimal) Decimal {
	baseExp := min(d.exp, d2.exp)
	rd := d.rescale(baseExp)
	rd2 := d2.rescale(baseExp)

	d3Value := new(big.Int).Add(rd.value, rd2.value)
	return Decimal{
		value:      d3Value,
		exp:        baseExp,
		fracDigits: fracDigitsPlus(d.fracDigits, d2.fracDigits),
	}
}

// Sub returns d - d2.
func (d Decimal) Sub(d2 Decimal) Decimal {
	baseExp := min(d.exp, d2.exp)
	rd := d.rescale(baseExp)
	rd2 := d2.rescale(baseExp)

	d3Value := new(big.Int).Sub(rd.value, rd2.value)
	return Decimal{
		value:      d3Value,
		exp:        baseExp,
		fracDigits: fracDigitsPlus(d.fracDigits, d2.fracDigits),
	}
}

// Mul returns d * d2.
func (d Decimal) Mul(d2 Decimal) Decimal {
	d.ensureInitialized()
	d2.ensureInitialized()

	expInt64 := int64(d.exp) + int64(d2.exp)
	if expInt64 > math.MaxInt32 || expInt64 < math.MinInt32 {
		// It is better to panic than to give incorrect results, as
		// decimals are usually used for money.
		panic(fmt.Sprintf("exponent %v overflows an int32!", expInt64))
	}

	d3Value := new(big.Int).Mul(d.value, d2.value)
	val := Decimal{
		value:      d3Value,
		exp:        int32(expInt64),
		fracDigits: fracDigitsMul(d.fracDigits, d2.fracDigits),
	}
	if val.exp < -(MaxFractionDigits) {
		val = val.Round(MaxFractionDigits)
	}
	return val
}

// Div returns d / d2. If it doesn't divide exactly, the result will have
// DivisionPrecision digits after the decimal point.
func (d Decimal) Div(d2 Decimal) Decimal {
	// Division is hard, use Rat to do it.
	ratNum := d.Rat()
	ratDenom := d2.Rat()

	quoRat := big.NewRat(0, 1).Quo(ratNum, ratDenom)

	// Converting from Rat to Decimal inefficiently for now.
	ret, err := ParseDecimal(quoRat.FloatString(MaxFractionDigits + 1))
	if err != nil {
		panic(err) // This should never happen.
	}
	// To pass test "2 / 3 * 3 < 2" -> "1".
	ret = ret.Truncate(MaxFractionDigits)
	ret.fracDigits = fracDigitsDiv(d.fracDigits)
	return ret
}

// Cmp compares the numbers represented by d and d2, and returns:
//
//     -1 if d <  d2
//      0 if d == d2
//     +1 if d >  d2
//
func (d Decimal) Cmp(d2 Decimal) int {
	baseExp := min(d.exp, d2.exp)
	rd := d.rescale(baseExp)
	rd2 := d2.rescale(baseExp)

	return rd.value.Cmp(rd2.value)
}

// Equals returns whether the numbers represented by d and d2 are equal.
func (d Decimal) Equals(d2 Decimal) bool {
	return d.Cmp(d2) == 0
}

// Exponent returns the exponent, or scale component of the decimal.
func (d Decimal) Exponent() int32 {
	return d.exp
}

// FracDigits returns the number of fractional digits of the decimal.
func (d Decimal) FracDigits() int32 {
	return d.fracDigits
}

// IntPart returns the integer component of the decimal.
func (d Decimal) IntPart() int64 {
	scaledD := d.rescale(0)
	return scaledD.value.Int64()
}

// Rat returns a rational number representation of the decimal.
func (d Decimal) Rat() *big.Rat {
	d.ensureInitialized()
	if d.exp <= 0 {
		// It must negate after casting to prevent int32 overflow.
		denom := new(big.Int).Exp(tenInt, big.NewInt(-int64(d.exp)), nil)
		return new(big.Rat).SetFrac(d.value, denom)
	}

	mul := new(big.Int).Exp(tenInt, big.NewInt(int64(d.exp)), nil)
	num := new(big.Int).Mul(d.value, mul)
	return new(big.Rat).SetFrac(num, oneInt)
}

// Float64 returns the nearest float64 value for d and a bool indicating
// whether f represents d exactly.
// For more details, see the documentation for big.Rat.Float64.
func (d Decimal) Float64() (f float64, exact bool) {
	return d.Rat().Float64()
}

// String returns the string representation of the decimal
// with the fixed point.
//
// Example:
//
//     d := New(-12345, -3)
//     println(d.String())
//
// Output:
//
//     -12.345
//
func (d Decimal) String() string {
	return d.StringFixed(d.fracDigits)
}

// StringFixed returns a rounded fixed-point string with places digits after
// the decimal point.
//
// Example:
//
// 	   NewFromFloat(0).StringFixed(2) // output: "0.00"
// 	   NewFromFloat(0).StringFixed(0) // output: "0"
// 	   NewFromFloat(5.45).StringFixed(0) // output: "5"
// 	   NewFromFloat(5.45).StringFixed(1) // output: "5.5"
// 	   NewFromFloat(5.45).StringFixed(2) // output: "5.45"
// 	   NewFromFloat(5.45).StringFixed(3) // output: "5.450"
// 	   NewFromFloat(545).StringFixed(-1) // output: "550"
//
func (d Decimal) StringFixed(places int32) string {
	rounded := d.Round(places)
	return rounded.string(false)
}

// Round rounds the decimal to places decimal places.
// If places < 0, it will round the integer part to the nearest 10^(-places).
//
// Example:
//
// 	   NewFromFloat(5.45).Round(1).String() // output: "5.5"
// 	   NewFromFloat(545).Round(-1).String() // output: "550"
//
func (d Decimal) Round(places int32) Decimal {
	// Truncate to places + 1.
	ret := d.rescale(-places - 1)

	// Add sign(d) * 0.5.
	if ret.value.Sign() < 0 {
		ret.value.Sub(ret.value, fiveInt)
	} else {
		ret.value.Add(ret.value, fiveInt)
	}

	// Floor for positive numbers, Ceil for negative numbers.
	_, m := ret.value.DivMod(ret.value, tenInt, new(big.Int))
	ret.exp++
	if ret.value.Sign() < 0 && m.Cmp(zeroInt) != 0 {
		ret.value.Add(ret.value, oneInt)
	}
	ret.fracDigits = places
	return ret
}

// Floor returns the nearest integer value less than or equal to d.
func (d Decimal) Floor() Decimal {
	d.ensureInitialized()

	exp := big.NewInt(10)

	// It must negate after casting to prevent int32 overflow.
	exp.Exp(exp, big.NewInt(-int64(d.exp)), nil)

	z := new(big.Int).Div(d.value, exp)
	return Decimal{value: z, exp: 0}
}

// Ceil returns the nearest integer value greater than or equal to d.
func (d Decimal) Ceil() Decimal {
	d.ensureInitialized()

	exp := big.NewInt(10)

	// It must negate after casting to prevent int32 overflow.
	exp.Exp(exp, big.NewInt(-int64(d.exp)), nil)

	z, m := new(big.Int).DivMod(d.value, exp, new(big.Int))
	if m.Cmp(zeroInt) != 0 {
		z.Add(z, oneInt)
	}
	return Decimal{value: z, exp: 0}
}

// Truncate truncates off digits from the number, without rounding.
//
// NOTE: precision is the last digit that will not be truncated (must be >= 0).
//
// Example:
//
//     decimal.NewFromString("123.456").Truncate(2).String() // "123.45"
//
func (d Decimal) Truncate(precision int32) Decimal {
	d.ensureInitialized()
	if precision >= 0 && -precision > d.exp {
		d = d.rescale(-precision)
	}
	d.fracDigits = precision
	return d
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (d *Decimal) UnmarshalJSON(decimalBytes []byte) error {
	str, err := unquoteIfQuoted(decimalBytes)
	if err != nil {
		return fmt.Errorf("Error decoding string '%s': %s", decimalBytes, err)
	}

	decimal, err := ParseDecimal(str)
	*d = decimal
	if err != nil {
		return fmt.Errorf("Error decoding string '%s': %s", str, err)
	}
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (d Decimal) MarshalJSON() ([]byte, error) {
	str := "\"" + d.String() + "\""
	return []byte(str), nil
}

// Scan implements the sql.Scanner interface for database deserialization.
func (d *Decimal) Scan(value interface{}) error {
	str, err := unquoteIfQuoted(value)
	if err != nil {
		return err
	}
	*d, err = ParseDecimal(str)

	return err
}

// Value implements the driver.Valuer interface for database serialization.
func (d Decimal) Value() (driver.Value, error) {
	return d.String(), nil
}

// BigIntValue returns the *bit.Int value member of decimal.
func (d Decimal) BigIntValue() *big.Int {
	return d.value
}

// UnmarshalText implements the encoding.TextUnmarshaler interface for XML
// deserialization.
func (d *Decimal) UnmarshalText(text []byte) error {
	str := string(text)

	dec, err := ParseDecimal(str)
	*d = dec
	if err != nil {
		return fmt.Errorf("Error decoding string '%s': %s", str, err)
	}

	return nil
}

// MarshalText implements the encoding.TextMarshaler interface for XML
// serialization.
func (d Decimal) MarshalText() (text []byte, err error) {
	return []byte(d.String()), nil
}

// StringScaled first scales the decimal then calls .String() on it.
// NOTE: buggy, unintuitive, and DEPRECATED! Use StringFixed instead.
func (d Decimal) StringScaled(exp int32) string {
	return d.rescale(exp).String()
}

func (d Decimal) string(trimTrailingZeros bool) string {
	if d.exp >= 0 {
		return d.rescale(0).value.String()
	}

	abs := new(big.Int).Abs(d.value)
	str := abs.String()

	var intPart, fractionalPart string

	// this cast to int will cause bugs if d.exp == INT_MIN
	// and you are on a 32-bit machine. Won't fix this super-edge case.
	dExpInt := int(d.exp)
	if len(str) > -dExpInt {
		intPart = str[:len(str)+dExpInt]
		fractionalPart = str[len(str)+dExpInt:]
	} else {
		intPart = "0"

		num0s := -dExpInt - len(str)
		fractionalPart = strings.Repeat("0", num0s) + str
	}

	if trimTrailingZeros {
		i := len(fractionalPart) - 1
		for ; i >= 0; i-- {
			if fractionalPart[i] != '0' {
				break
			}
		}
		fractionalPart = fractionalPart[:i+1]
	}

	number := intPart
	if len(fractionalPart) > 0 {
		number += "." + fractionalPart
	}

	if d.value.Sign() < 0 {
		return "-" + number
	}

	return number
}

func (d *Decimal) ensureInitialized() {
	if d.value == nil {
		d.value = new(big.Int)
	}
}

func min(x, y int32) int32 {
	if x >= y {
		return y
	}
	return x
}

func max(x, y int32) int32 {
	if x >= y {
		return x
	}
	return y
}

func round(n float64) int64 {
	if n < 0 {
		return int64(n - 0.5)
	}
	return int64(n + 0.5)
}

func unquoteIfQuoted(value interface{}) (string, error) {
	bytes, ok := value.([]byte)
	if !ok {
		return "", fmt.Errorf("Could not convert value '%+v' to byte array",
			value)
	}

	// If the amount is quoted, strip the quotes.
	if len(bytes) > 2 && bytes[0] == '"' && bytes[len(bytes)-1] == '"' {
		bytes = bytes[1 : len(bytes)-1]
	}
	return string(bytes), nil
}

func fracDigitsDefault(exp int32) int32 {
	if exp < 0 {
		return min(MaxFractionDigits, -exp)
	}

	return 0
}

func fracDigitsPlus(x, y int32) int32 {
	return max(x, y)
}

func fracDigitsDiv(x int32) int32 {
	return min(x+DivIncreasePrecision, MaxFractionDigits)
}

func fracDigitsMul(a, b int32) int32 {
	return min(MaxFractionDigits, a+b)
}
