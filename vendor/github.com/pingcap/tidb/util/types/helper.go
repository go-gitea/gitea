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

	"github.com/juju/errors"
	"github.com/pingcap/tidb/mysql"
)

// RoundFloat rounds float val to the nearest integer value with float64 format, like GNU rint function.
// RoundFloat uses default rounding mode, see http://www.gnu.org/software/libc/manual/html_node/Rounding.html
// so we will choose the even number if the result is midway between two representable value.
// e.g, 1.5 -> 2, 2.5 -> 2.
func RoundFloat(f float64) float64 {
	if math.Remainder(f, 1.0) < 0 {
		return math.Ceil(f)
	}
	return math.Floor(f)
}

func getMaxFloat(flen int, decimal int) float64 {
	intPartLen := flen - decimal
	f := math.Pow10(intPartLen)
	f -= math.Pow10(-decimal)
	return f
}

func truncateFloat(f float64, decimal int) float64 {
	pow := math.Pow10(decimal)
	t := (f - math.Floor(f)) * pow

	round := RoundFloat(t)

	f = math.Floor(f) + round/pow
	return f
}

// TruncateFloat tries to truncate f.
// If the result exceeds the max/min float that flen/decimal allowed, returns the max/min float allowed.
func TruncateFloat(f float64, flen int, decimal int) (float64, error) {
	if math.IsNaN(f) {
		// nan returns 0
		return 0, nil
	}

	maxF := getMaxFloat(flen, decimal)

	if !math.IsInf(f, 0) {
		f = truncateFloat(f, decimal)
	}

	if f > maxF {
		f = maxF
	} else if f < -maxF {
		f = -maxF
	}

	return f, nil
}

// CalculateSum adds v to sum.
func CalculateSum(sum interface{}, v interface{}) (interface{}, error) {
	// for avg and sum calculation
	// avg and sum use decimal for integer and decimal type, use float for others
	// see https://dev.mysql.com/doc/refman/5.7/en/group-by-functions.html
	var (
		data interface{}
		err  error
	)

	switch y := v.(type) {
	case int, uint, int8, uint8, int16, uint16, int32, uint32, int64, uint64:
		data, err = mysql.ConvertToDecimal(v)
	case mysql.Decimal:
		data = y
	case nil:
		data = nil
	default:
		data, err = ToFloat64(v)
	}

	if err != nil {
		return nil, errors.Trace(err)
	}
	if data == nil {
		return sum, nil
	}
	switch x := sum.(type) {
	case nil:
		return data, nil
	case float64:
		return x + data.(float64), nil
	case mysql.Decimal:
		return x.Add(data.(mysql.Decimal)), nil
	default:
		return nil, errors.Errorf("invalid value %v(%T) for aggregate", x, x)
	}
}
