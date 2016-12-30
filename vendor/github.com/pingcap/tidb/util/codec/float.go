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
	"math"

	"github.com/juju/errors"
)

func encodeFloatToCmpUint64(f float64) uint64 {
	u := math.Float64bits(f)
	if f >= 0 {
		u |= signMask
	} else {
		u = ^u
	}
	return u
}

func decodeCmpUintToFloat(u uint64) float64 {
	if u&signMask > 0 {
		u &= ^signMask
	} else {
		u = ^u
	}
	return math.Float64frombits(u)
}

// EncodeFloat encodes a float v into a byte slice which can be sorted lexicographically later.
// EncodeFloat guarantees that the encoded value is in ascending order for comparison.
func EncodeFloat(b []byte, v float64) []byte {
	u := encodeFloatToCmpUint64(v)
	return EncodeUint(b, u)
}

// DecodeFloat decodes a float from a byte slice generated with EncodeFloat before.
func DecodeFloat(b []byte) ([]byte, float64, error) {
	b, u, err := DecodeUint(b)
	return b, decodeCmpUintToFloat(u), errors.Trace(err)
}

// EncodeFloatDesc encodes a float v into a byte slice which can be sorted lexicographically later.
// EncodeFloatDesc guarantees that the encoded value is in descending order for comparison.
func EncodeFloatDesc(b []byte, v float64) []byte {
	u := encodeFloatToCmpUint64(v)
	return EncodeUintDesc(b, u)
}

// DecodeFloatDesc decodes a float from a byte slice generated with EncodeFloatDesc before.
func DecodeFloatDesc(b []byte) ([]byte, float64, error) {
	b, u, err := DecodeUintDesc(b)
	return b, decodeCmpUintToFloat(u), errors.Trace(err)
}
