// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"bytes"
	"strings"
)

// OptionalBool a boolean that can be "null"
type OptionalBool byte

const (
	// OptionalBoolNone a "null" boolean value
	OptionalBoolNone = iota
	// OptionalBoolTrue a "true" boolean value
	OptionalBoolTrue
	// OptionalBoolFalse a "false" boolean value
	OptionalBoolFalse
)

// IsTrue return true if equal to OptionalBoolTrue
func (o OptionalBool) IsTrue() bool {
	return o == OptionalBoolTrue
}

// IsFalse return true if equal to OptionalBoolFalse
func (o OptionalBool) IsFalse() bool {
	return o == OptionalBoolFalse
}

// IsNone return true if equal to OptionalBoolNone
func (o OptionalBool) IsNone() bool {
	return o == OptionalBoolNone
}

// OptionalBoolOf get the corresponding OptionalBool of a bool
func OptionalBoolOf(b bool) OptionalBool {
	if b {
		return OptionalBoolTrue
	}
	return OptionalBoolFalse
}

// Max max of two ints
func Max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

// Min min of two ints
func Min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

// IsEmptyString checks if the provided string is empty
func IsEmptyString(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}

// NormalizeEOL will convert Windows (CRLF) and Mac (CR) EOLs to UNIX (LF)
func NormalizeEOL(input []byte) []byte {
	var right, left, pos int
	if right = bytes.IndexByte(input, '\r'); right == -1 {
		return input
	}
	length := len(input)
	tmp := make([]byte, length)

	// We know that left < length because otherwise right would be -1 from IndexByte.
	copy(tmp[pos:pos+right], input[left:left+right])
	pos += right
	tmp[pos] = '\n'
	left += right + 1
	pos++

	for left < length {
		if input[left] == '\n' {
			left++
		}

		right = bytes.IndexByte(input[left:], '\r')
		if right == -1 {
			copy(tmp[pos:], input[left:])
			pos += length - left
			break
		}
		copy(tmp[pos:pos+right], input[left:left+right])
		pos += right
		tmp[pos] = '\n'
		left += right + 1
		pos++
	}
	return tmp[:pos]
}
