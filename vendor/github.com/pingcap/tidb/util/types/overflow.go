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
)

// ErrArithOverflow is the error for arthimetic operation overflow.
var ErrArithOverflow = errors.New("operation overflow")

// AddUint64 adds uint64 a and b if no overflow, else returns error.
func AddUint64(a uint64, b uint64) (uint64, error) {
	if math.MaxUint64-a < b {
		return 0, errors.Trace(ErrArithOverflow)
	}
	return a + b, nil
}

// AddInt64 adds int64 a and b if no overflow, otherwise returns error.
func AddInt64(a int64, b int64) (int64, error) {
	if (a > 0 && b > 0 && math.MaxInt64-a < b) ||
		(a < 0 && b < 0 && math.MinInt64-a > b) {
		return 0, errors.Trace(ErrArithOverflow)
	}

	return a + b, nil
}

// AddInteger adds uint64 a and int64 b and returns uint64 if no overflow error.
func AddInteger(a uint64, b int64) (uint64, error) {
	if b >= 0 {
		return AddUint64(a, uint64(b))
	}

	if uint64(-b) > a {
		return 0, errors.Trace(ErrArithOverflow)
	}
	return a - uint64(-b), nil
}

// SubUint64 substracts uint64 a with b and returns uint64 if no overflow error.
func SubUint64(a uint64, b uint64) (uint64, error) {
	if a < b {
		return 0, errors.Trace(ErrArithOverflow)
	}
	return a - b, nil
}

// SubInt64 substracts int64 a with b and returns int64 if no overflow error.
func SubInt64(a int64, b int64) (int64, error) {
	if (a > 0 && b < 0 && math.MaxInt64-a < -b) ||
		(a < 0 && b > 0 && math.MinInt64-a > -b) ||
		(a == 0 && b == math.MinInt64) {
		return 0, errors.Trace(ErrArithOverflow)
	}
	return a - b, nil
}

// SubUintWithInt substracts uint64 a with int64 b and returns uint64 if no overflow error.
func SubUintWithInt(a uint64, b int64) (uint64, error) {
	if b < 0 {
		return AddUint64(a, uint64(-b))
	}
	return SubUint64(a, uint64(b))
}

// SubIntWithUint substracts int64 a with uint64 b and returns uint64 if no overflow error.
func SubIntWithUint(a int64, b uint64) (uint64, error) {
	if a < 0 || uint64(a) < b {
		return 0, errors.Trace(ErrArithOverflow)
	}
	return uint64(a) - b, nil
}

// MulUint64 multiplies uint64 a and b and returns uint64 if no overflow error.
func MulUint64(a uint64, b uint64) (uint64, error) {
	if b > 0 && a > math.MaxUint64/b {
		return 0, errors.Trace(ErrArithOverflow)
	}
	return a * b, nil
}

// MulInt64 multiplies int64 a and b and returns int64 if no overflow error.
func MulInt64(a int64, b int64) (int64, error) {
	if a == 0 || b == 0 {
		return 0, nil
	}

	var (
		res      uint64
		err      error
		negative = false
	)

	if a > 0 && b > 0 {
		res, err = MulUint64(uint64(a), uint64(b))
	} else if a < 0 && b < 0 {
		res, err = MulUint64(uint64(-a), uint64(-b))
	} else if a < 0 && b > 0 {
		negative = true
		res, err = MulUint64(uint64(-a), uint64(b))
	} else {
		negative = true
		res, err = MulUint64(uint64(a), uint64(-b))
	}

	if err != nil {
		return 0, errors.Trace(err)
	}

	if negative {
		// negative result
		if res > math.MaxInt64+1 {
			return 0, errors.Trace(ErrArithOverflow)
		}

		return -int64(res), nil
	}

	// positive result
	if res > math.MaxInt64 {
		return 0, errors.Trace(ErrArithOverflow)
	}

	return int64(res), nil
}

// MulInteger multiplies uint64 a and int64 b, and returns uint64 if no overflow error.
func MulInteger(a uint64, b int64) (uint64, error) {
	if a == 0 || b == 0 {
		return 0, nil
	}

	if b < 0 {
		return 0, errors.Trace(ErrArithOverflow)
	}

	return MulUint64(a, uint64(b))
}

// DivInt64 divides int64 a with b, returns int64 if no overflow error.
// It just checks overflow, if b is zero, a "divide by zero" panic throws.
func DivInt64(a int64, b int64) (int64, error) {
	if a == math.MinInt64 && b == -1 {
		return 0, errors.Trace(ErrArithOverflow)
	}

	return a / b, nil
}

// DivUintWithInt divides uint64 a with int64 b, returns uint64 if no overflow error.
// It just checks overflow, if b is zero, a "divide by zero" panic throws.
func DivUintWithInt(a uint64, b int64) (uint64, error) {
	if b < 0 {
		if a != 0 && uint64(-b) <= a {
			return 0, errors.Trace(ErrArithOverflow)
		}

		return 0, nil
	}

	return a / uint64(b), nil
}

// DivIntWithUint divides int64 a with uint64 b, returns uint64 if no overflow error.
// It just checks overflow, if b is zero, a "divide by zero" panic throws.
func DivIntWithUint(a int64, b uint64) (uint64, error) {
	if a < 0 {
		if uint64(-a) >= b {
			return 0, errors.Trace(ErrArithOverflow)
		}

		return 0, nil
	}

	return uint64(a) / b, nil
}
