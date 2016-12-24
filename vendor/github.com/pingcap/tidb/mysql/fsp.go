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

import (
	"math"
	"strconv"
	"strings"

	"github.com/juju/errors"
)

const (
	// UnspecifiedFsp is the unspecified fractional seconds part.
	UnspecifiedFsp int = -1
	// MaxFsp is the maximum digit of fractional seconds part.
	MaxFsp int = 6
	// MinFsp is the minimum digit of fractional seconds part.
	MinFsp int = 0
	// DefaultFsp is the default digit of fractional seconds part.
	// MySQL use 0 as the default Fsp.
	DefaultFsp int = 0
)

func checkFsp(fsp int) (int, error) {
	if fsp == UnspecifiedFsp {
		return DefaultFsp, nil
	}
	if fsp < MinFsp || fsp > MaxFsp {
		return DefaultFsp, errors.Errorf("Invalid fsp %d", fsp)
	}
	return fsp, nil
}

func parseFrac(s string, fsp int) (int, error) {
	if len(s) == 0 {
		return 0, nil
	}

	var err error
	fsp, err = checkFsp(fsp)
	if err != nil {
		return 0, errors.Trace(err)
	}

	// Use float to calculate frac, e.g, "123" -> "0.123"
	if !strings.HasPrefix(s, ".") && !strings.HasPrefix(s, "0.") {
		s = "0." + s
	}

	frac, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, errors.Trace(err)
	}

	// round frac to the nearest value with FSP
	var round float64
	pow := math.Pow(10, float64(fsp))
	digit := pow * frac
	_, div := math.Modf(digit)
	if div >= 0.5 {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}

	// Get the final frac, with 6 digit number
	//  0.1236 round 3 -> 124 -> 123000
	//  0.0312 round 2 -> 3 -> 30000
	return int(round * math.Pow10(MaxFsp-fsp)), nil
}

// alignFrac is used to generate alignment frac, like `100` -> `100000`
func alignFrac(s string, fsp int) string {
	sl := len(s)
	if sl < fsp {
		return s + strings.Repeat("0", fsp-sl)
	}

	return s
}
