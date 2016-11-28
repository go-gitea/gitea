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
	"encoding/binary"
	"runtime"
	"unsafe"

	"github.com/juju/errors"
)

const (
	encGroupSize = 8
	encMarker    = byte(0xFF)
	encPad       = byte(0x0)
)

var (
	pads    = make([]byte, encGroupSize)
	encPads = []byte{encPad}
)

// EncodeBytes guarantees the encoded value is in ascending order for comparison,
// encoding with the following rule:
//  [group1][marker1]...[groupN][markerN]
//  group is 8 bytes slice which is padding with 0.
//  marker is `0xFF - padding 0 count`
// For example:
//   [] -> [0, 0, 0, 0, 0, 0, 0, 0, 247]
//   [1, 2, 3] -> [1, 2, 3, 0, 0, 0, 0, 0, 250]
//   [1, 2, 3, 0] -> [1, 2, 3, 0, 0, 0, 0, 0, 251]
//   [1, 2, 3, 4, 5, 6, 7, 8] -> [1, 2, 3, 4, 5, 6, 7, 8, 255, 0, 0, 0, 0, 0, 0, 0, 0, 247]
// Refer: https://github.com/facebook/mysql-5.6/wiki/MyRocks-record-format#memcomparable-format
func EncodeBytes(b []byte, data []byte) []byte {
	// Allocate more space to avoid unnecessary slice growing.
	// Assume that the byte slice size is about `(len(data) / encGroupSize + 1) * (encGroupSize + 1)` bytes,
	// that is `(len(data) / 8 + 1) * 9` in our implement.
	dLen := len(data)
	reallocSize := (dLen/encGroupSize + 1) * (encGroupSize + 1)
	result := reallocBytes(b, reallocSize)
	for idx := 0; idx <= dLen; idx += encGroupSize {
		remain := dLen - idx
		padCount := 0
		if remain >= encGroupSize {
			result = append(result, data[idx:idx+encGroupSize]...)
		} else {
			padCount = encGroupSize - remain
			result = append(result, data[idx:]...)
			result = append(result, pads[:padCount]...)
		}

		marker := encMarker - byte(padCount)
		result = append(result, marker)
	}

	return result
}

func decodeBytes(b []byte, reverse bool) ([]byte, []byte, error) {
	data := make([]byte, 0, len(b))
	for {
		if len(b) < encGroupSize+1 {
			return nil, nil, errors.New("insufficient bytes to decode value")
		}

		groupBytes := b[:encGroupSize+1]
		if reverse {
			reverseBytes(groupBytes)
		}

		group := groupBytes[:encGroupSize]
		marker := groupBytes[encGroupSize]

		// Check validity of marker.
		padCount := encMarker - marker
		realGroupSize := encGroupSize - padCount
		if padCount > encGroupSize {
			return nil, nil, errors.Errorf("invalid marker byte, group bytes %q", groupBytes)
		}

		data = append(data, group[:realGroupSize]...)
		b = b[encGroupSize+1:]

		if marker != encMarker {
			// Check validity of padding bytes.
			if bytes.Count(group[realGroupSize:], encPads) != int(padCount) {
				return nil, nil, errors.Errorf("invalid padding byte, group bytes %q", groupBytes)
			}

			break
		}
	}

	return b, data, nil
}

// DecodeBytes decodes bytes which is encoded by EncodeBytes before,
// returns the leftover bytes and decoded value if no error.
func DecodeBytes(b []byte) ([]byte, []byte, error) {
	return decodeBytes(b, false)
}

// EncodeBytesDesc first encodes bytes using EncodeBytes, then bitwise reverses
// encoded value to guarantee the encoded value is in descending order for comparison.
func EncodeBytesDesc(b []byte, data []byte) []byte {
	n := len(b)
	b = EncodeBytes(b, data)
	reverseBytes(b[n:])
	return b
}

// DecodeBytesDesc decodes bytes which is encoded by EncodeBytesDesc before,
// returns the leftover bytes and decoded value if no error.
func DecodeBytesDesc(b []byte) ([]byte, []byte, error) {
	return decodeBytes(b, true)
}

// EncodeCompactBytes joins bytes with its length into a byte slice. It is more
// efficient in both space and time compare to EncodeBytes. Note that the encoded
// result is not memcomparable.
func EncodeCompactBytes(b []byte, data []byte) []byte {
	b = reallocBytes(b, binary.MaxVarintLen64+len(data))
	b = EncodeVarint(b, int64(len(data)))
	return append(b, data...)
}

// DecodeCompactBytes decodes bytes which is encoded by EncodeCompactBytes before.
func DecodeCompactBytes(b []byte) ([]byte, []byte, error) {
	b, n, err := DecodeVarint(b)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	if int64(len(b)) < n {
		return nil, nil, errors.Errorf("insufficient bytes to decode value, expected length: %v", n)
	}
	return b[n:], b[:n], nil
}

// See https://golang.org/src/crypto/cipher/xor.go
const wordSize = int(unsafe.Sizeof(uintptr(0)))
const supportsUnaligned = runtime.GOARCH == "386" || runtime.GOARCH == "amd64"

func fastReverseBytes(b []byte) {
	n := len(b)
	w := n / wordSize
	if w > 0 {
		bw := *(*[]uintptr)(unsafe.Pointer(&b))
		for i := 0; i < w; i++ {
			bw[i] = ^bw[i]
		}
	}

	for i := w * wordSize; i < n; i++ {
		b[i] = ^b[i]
	}
}

func safeReverseBytes(b []byte) {
	for i := range b {
		b[i] = ^b[i]
	}
}

func reverseBytes(b []byte) {
	if supportsUnaligned {
		fastReverseBytes(b)
		return
	}

	safeReverseBytes(b)
}

// like realloc.
func reallocBytes(b []byte, n int) []byte {
	newSize := len(b) + n
	if cap(b) < newSize {
		bs := make([]byte, len(b), newSize)
		copy(bs, b)
		return bs
	}

	// slice b has capability to store n bytes
	return b
}
