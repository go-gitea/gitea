// Copyright 2014 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// This code originated from:
// https://github.com/cockroachdb/cockroach/blob/2dd65dde5d90c157f4b93f92502ca1063b904e1d/pkg/util/encoding/encoding.go

// Modified to not use pkg/errors

package segment

import (
	"errors"
	"fmt"
)

const (
	MaxVarintSize = 9

	// IntMin is chosen such that the range of int tags does not overlap the
	// ascii character set that is frequently used in testing.
	IntMin      = 0x80 // 128
	intMaxWidth = 8
	intZero     = IntMin + intMaxWidth           // 136
	intSmall    = IntMax - intZero - intMaxWidth // 109
	// IntMax is the maximum int tag value.
	IntMax = 0xfd // 253
)

// EncodeUvarintAscending encodes the uint64 value using a variable length
// (length-prefixed) representation. The length is encoded as a single
// byte indicating the number of encoded bytes (-8) to follow. See
// EncodeVarintAscending for rationale. The encoded bytes are appended to the
// supplied buffer and the final buffer is returned.
func EncodeUvarintAscending(b []byte, v uint64) []byte {
	switch {
	case v <= intSmall:
		return append(b, intZero+byte(v))
	case v <= 0xff:
		return append(b, IntMax-7, byte(v))
	case v <= 0xffff:
		return append(b, IntMax-6, byte(v>>8), byte(v))
	case v <= 0xffffff:
		return append(b, IntMax-5, byte(v>>16), byte(v>>8), byte(v))
	case v <= 0xffffffff:
		return append(b, IntMax-4, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	case v <= 0xffffffffff:
		return append(b, IntMax-3, byte(v>>32), byte(v>>24), byte(v>>16), byte(v>>8),
			byte(v))
	case v <= 0xffffffffffff:
		return append(b, IntMax-2, byte(v>>40), byte(v>>32), byte(v>>24), byte(v>>16),
			byte(v>>8), byte(v))
	case v <= 0xffffffffffffff:
		return append(b, IntMax-1, byte(v>>48), byte(v>>40), byte(v>>32), byte(v>>24),
			byte(v>>16), byte(v>>8), byte(v))
	default:
		return append(b, IntMax, byte(v>>56), byte(v>>48), byte(v>>40), byte(v>>32),
			byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	}
}

// DecodeUvarintAscending decodes a varint encoded uint64 from the input
// buffer. The remainder of the input buffer and the decoded uint64
// are returned.
func DecodeUvarintAscending(b []byte) ([]byte, uint64, error) {
	if len(b) == 0 {
		return nil, 0, fmt.Errorf("insufficient bytes to decode uvarint value")
	}
	length := int(b[0]) - intZero
	b = b[1:] // skip length byte
	if length <= intSmall {
		return b, uint64(length), nil
	}
	length -= intSmall
	if length < 0 || length > 8 {
		return nil, 0, fmt.Errorf("invalid uvarint length of %d", length)
	} else if len(b) < length {
		return nil, 0, fmt.Errorf("insufficient bytes to decode uvarint value: %q", b)
	}
	var v uint64
	// It is faster to range over the elements in a slice than to index
	// into the slice on each loop iteration.
	for _, t := range b[:length] {
		v = (v << 8) | uint64(t)
	}
	return b[length:], v, nil
}

// ------------------------------------------------------------

type MemUvarintReader struct {
	C int // index of next byte to read from S
	S []byte
}

func NewMemUvarintReader(s []byte) *MemUvarintReader {
	return &MemUvarintReader{S: s}
}

// Len returns the number of unread bytes.
func (r *MemUvarintReader) Len() int {
	n := len(r.S) - r.C
	if n < 0 {
		return 0
	}
	return n
}

var ErrMemUvarintReaderOverflow = errors.New("MemUvarintReader overflow")

// ReadUvarint reads an encoded uint64.  The original code this was
// based on is at encoding/binary/ReadUvarint().
func (r *MemUvarintReader) ReadUvarint() (uint64, error) {
	var x uint64
	var s uint
	var C = r.C
	var S = r.S

	for {
		b := S[C]
		C++

		if b < 0x80 {
			r.C = C

			// why 63?  The original code had an 'i += 1' loop var and
			// checked for i > 9 || i == 9 ...; but, we no longer
			// check for the i var, but instead check here for s,
			// which is incremented by 7.  So, 7*9 == 63.
			//
			// why the "extra" >= check?  The normal case is that s <
			// 63, so we check this single >= guard first so that we
			// hit the normal, nil-error return pathway sooner.
			if s >= 63 && (s > 63 || s == 63 && b > 1) {
				return 0, ErrMemUvarintReaderOverflow
			}

			return x | uint64(b)<<s, nil
		}

		x |= uint64(b&0x7f) << s
		s += 7
	}
}

// SkipUvarint skips ahead one encoded uint64.
func (r *MemUvarintReader) SkipUvarint() {
	for {
		b := r.S[r.C]
		r.C++

		if b < 0x80 {
			return
		}
	}
}

// SkipBytes skips a count number of bytes.
func (r *MemUvarintReader) SkipBytes(count int) {
	r.C = r.C + count
}

func (r *MemUvarintReader) Reset(s []byte) {
	r.C = 0
	r.S = s
}
