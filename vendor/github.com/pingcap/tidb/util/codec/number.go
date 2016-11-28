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
	"encoding/binary"

	"github.com/juju/errors"
)

const signMask uint64 = 0x8000000000000000

func encodeIntToCmpUint(v int64) uint64 {
	u := uint64(v)
	if u&signMask > 0 {
		u &= ^signMask
	} else {
		u |= signMask
	}

	return u
}

func decodeCmpUintToInt(u uint64) int64 {
	if u&signMask > 0 {
		u &= ^signMask
	} else {
		u |= signMask
	}

	return int64(u)
}

// EncodeInt appends the encoded value to slice b and returns the appended slice.
// EncodeInt guarantees that the encoded value is in ascending order for comparison.
func EncodeInt(b []byte, v int64) []byte {
	var data [8]byte
	u := encodeIntToCmpUint(v)
	binary.BigEndian.PutUint64(data[:], u)
	return append(b, data[:]...)
}

// EncodeIntDesc appends the encoded value to slice b and returns the appended slice.
// EncodeIntDesc guarantees that the encoded value is in descending order for comparison.
func EncodeIntDesc(b []byte, v int64) []byte {
	var data [8]byte
	u := encodeIntToCmpUint(v)
	binary.BigEndian.PutUint64(data[:], ^u)
	return append(b, data[:]...)
}

// DecodeInt decodes value encoded by EncodeInt before.
// It returns the leftover un-decoded slice, decoded value if no error.
func DecodeInt(b []byte) ([]byte, int64, error) {
	if len(b) < 8 {
		return nil, 0, errors.New("insufficient bytes to decode value")
	}

	u := binary.BigEndian.Uint64(b[:8])
	v := decodeCmpUintToInt(u)
	b = b[8:]
	return b, v, nil
}

// DecodeIntDesc decodes value encoded by EncodeInt before.
// It returns the leftover un-decoded slice, decoded value if no error.
func DecodeIntDesc(b []byte) ([]byte, int64, error) {
	if len(b) < 8 {
		return nil, 0, errors.New("insufficient bytes to decode value")
	}

	u := binary.BigEndian.Uint64(b[:8])
	v := decodeCmpUintToInt(^u)
	b = b[8:]
	return b, v, nil
}

// EncodeUint appends the encoded value to slice b and returns the appended slice.
// EncodeUint guarantees that the encoded value is in ascending order for comparison.
func EncodeUint(b []byte, v uint64) []byte {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], v)
	return append(b, data[:]...)
}

// EncodeUintDesc appends the encoded value to slice b and returns the appended slice.
// EncodeUintDesc guarantees that the encoded value is in descending order for comparison.
func EncodeUintDesc(b []byte, v uint64) []byte {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], ^v)
	return append(b, data[:]...)
}

// DecodeUint decodes value encoded by EncodeUint before.
// It returns the leftover un-decoded slice, decoded value if no error.
func DecodeUint(b []byte) ([]byte, uint64, error) {
	if len(b) < 8 {
		return nil, 0, errors.New("insufficient bytes to decode value")
	}

	v := binary.BigEndian.Uint64(b[:8])
	b = b[8:]
	return b, v, nil
}

// DecodeUintDesc decodes value encoded by EncodeInt before.
// It returns the leftover un-decoded slice, decoded value if no error.
func DecodeUintDesc(b []byte) ([]byte, uint64, error) {
	if len(b) < 8 {
		return nil, 0, errors.New("insufficient bytes to decode value")
	}

	data := b[:8]
	v := binary.BigEndian.Uint64(data)
	b = b[8:]
	return b, ^v, nil
}

// EncodeVarint appends the encoded value to slice b and returns the appended slice.
// Note that the encoded result is not memcomparable.
func EncodeVarint(b []byte, v int64) []byte {
	var data [binary.MaxVarintLen64]byte
	n := binary.PutVarint(data[:], v)
	return append(b, data[:n]...)
}

// DecodeVarint decodes value encoded by EncodeVarint before.
// It returns the leftover un-decoded slice, decoded value if no error.
func DecodeVarint(b []byte) ([]byte, int64, error) {
	v, n := binary.Varint(b)
	if n > 0 {
		return b[n:], v, nil
	}
	if n < 0 {
		return nil, 0, errors.New("value larger than 64 bits")
	}
	return nil, 0, errors.New("insufficient bytes to decode value")
}

// EncodeUvarint appends the encoded value to slice b and returns the appended slice.
// Note that the encoded result is not memcomparable.
func EncodeUvarint(b []byte, v uint64) []byte {
	var data [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(data[:], v)
	return append(b, data[:n]...)
}

// DecodeUvarint decodes value encoded by EncodeUvarint before.
// It returns the leftover un-decoded slice, decoded value if no error.
func DecodeUvarint(b []byte) ([]byte, uint64, error) {
	v, n := binary.Uvarint(b)
	if n > 0 {
		return b[n:], v, nil
	}
	if n < 0 {
		return nil, 0, errors.New("value larger than 64 bits")
	}
	return nil, 0, errors.New("insufficient bytes to decode value")
}
