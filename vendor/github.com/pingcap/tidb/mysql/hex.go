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
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/juju/errors"
)

// Hex is for mysql hexadecimal literal type.
type Hex struct {
	// Value holds numeric value for hexadecimal literal.
	Value int64
}

// String implements fmt.Stringer interface.
func (h Hex) String() string {
	s := fmt.Sprintf("%X", h.Value)
	if len(s)%2 != 0 {
		return "0x0" + s
	}

	return "0x" + s
}

// ToNumber changes hexadecimal type to float64 for numeric operation.
// MySQL treats hexadecimal literal as double type.
func (h Hex) ToNumber() float64 {
	return float64(h.Value)
}

// ToString returns the string representation for hexadecimal literal.
func (h Hex) ToString() string {
	s := fmt.Sprintf("%x", h.Value)
	if len(s)%2 != 0 {
		s = "0" + s
	}

	// should never error.
	b, _ := hex.DecodeString(s)
	return string(b)
}

// ParseHex parses hexadecimal literal string.
// The string format can be X'val', x'val' or 0xval.
// val must in (0...9, a...z, A...Z).
func ParseHex(s string) (Hex, error) {
	if len(s) == 0 {
		return Hex{}, errors.Errorf("invalid empty string for parsing hexadecimal literal")
	}

	if s[0] == 'x' || s[0] == 'X' {
		// format is x'val' or X'val'
		s = strings.Trim(s[1:], "'")
		if len(s)%2 != 0 {
			return Hex{}, errors.Errorf("invalid hexadecimal format, must even numbers, but %d", len(s))
		}
		s = "0x" + s
	} else if !strings.HasPrefix(s, "0x") {
		// here means format is not x'val', X'val' or 0xval.
		return Hex{}, errors.Errorf("invalid hexadecimal format %s", s)
	}

	n, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return Hex{}, errors.Trace(err)
	}

	return Hex{Value: n}, nil
}
