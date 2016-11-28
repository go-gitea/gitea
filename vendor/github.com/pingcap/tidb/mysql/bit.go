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
	"fmt"
	"strconv"
	"strings"

	"github.com/juju/errors"
)

// Bit is for mysql bit type.
type Bit struct {
	// Value holds the value for bit type.
	Value uint64

	// Width is the display with for bit value.
	// e.g, with is 8, 0 is for 0b00000000.
	Width int
}

// String implements fmt.Stringer interface.
func (b Bit) String() string {
	format := fmt.Sprintf("0b%%0%db", b.Width)
	return fmt.Sprintf(format, b.Value)
}

// ToNumber changes bit type to float64 for numeric operation.
// MySQL treats bit as double type.
func (b Bit) ToNumber() float64 {
	return float64(b.Value)
}

// ToString returns the binary string for bit type.
func (b Bit) ToString() string {
	byteSize := (b.Width + 7) / 8
	buf := make([]byte, byteSize)
	for i := byteSize - 1; i >= 0; i-- {
		buf[byteSize-i-1] = byte(b.Value >> uint(i*8))
	}
	return string(buf)
}

// Min and Max bit width.
const (
	MinBitWidth = 1
	MaxBitWidth = 64
	// UnspecifiedBitWidth is the unspecified with if you want to calculate bit width dynamically.
	UnspecifiedBitWidth = -1
)

// ParseBit parses bit string.
// The string format can be b'val', B'val' or 0bval, val must be 0 or 1.
// Width is the display width for bit representation. -1 means calculating
// width dynamically, using following algorithm: (len("011101") + 7) & ^7,
// e.g, if bit string is 0b01, the above will return 8 for its bit width.
func ParseBit(s string, width int) (Bit, error) {
	if len(s) == 0 {
		return Bit{}, errors.Errorf("invalid empty string for parsing bit type")
	}

	if s[0] == 'b' || s[0] == 'B' {
		// format is b'val' or B'val'
		s = strings.Trim(s[1:], "'")
	} else if strings.HasPrefix(s, "0b") {
		s = s[2:]
	} else {
		// here means format is not b'val', B'val' or 0bval.
		return Bit{}, errors.Errorf("invalid bit type format %s", s)
	}

	if width == UnspecifiedBitWidth {
		width = (len(s) + 7) & ^7
	}

	if width == 0 {
		width = MinBitWidth
	}

	if width < MinBitWidth || width > MaxBitWidth {
		return Bit{}, errors.Errorf("invalid display width for bit type, must in [1, 64], but %d", width)
	}

	n, err := strconv.ParseUint(s, 2, 64)
	if err != nil {
		return Bit{}, errors.Trace(err)
	}

	if n > (uint64(1)<<uint64(width))-1 {
		return Bit{}, errors.Errorf("bit %s is too long for width %d", s, width)
	}

	return Bit{Value: n, Width: width}, nil
}
