// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package internal is a collection of common compression algorithms.
//
// For performance reasons, these packages lack strong error checking and
// require that the caller to ensure that strict invariants are kept.
package internal

var (
	// IdentityLUT returns the input key itself.
	IdentityLUT = func() (lut [256]byte) {
		for i := range lut {
			lut[i] = uint8(i)
		}
		return lut
	}()

	// ReverseLUT returns the input key with its bits reversed.
	ReverseLUT = func() (lut [256]byte) {
		for i := range lut {
			b := uint8(i)
			b = (b&0xaa)>>1 | (b&0x55)<<1
			b = (b&0xcc)>>2 | (b&0x33)<<2
			b = (b&0xf0)>>4 | (b&0x0f)<<4
			lut[i] = b
		}
		return lut
	}()
)

// ReverseUint32 reverses all bits of v.
func ReverseUint32(v uint32) (x uint32) {
	x |= uint32(ReverseLUT[byte(v>>0)]) << 24
	x |= uint32(ReverseLUT[byte(v>>8)]) << 16
	x |= uint32(ReverseLUT[byte(v>>16)]) << 8
	x |= uint32(ReverseLUT[byte(v>>24)]) << 0
	return x
}

// ReverseUint32N reverses the lower n bits of v.
func ReverseUint32N(v uint32, n uint) (x uint32) {
	return ReverseUint32(v << (32 - n))
}

// ReverseUint64 reverses all bits of v.
func ReverseUint64(v uint64) (x uint64) {
	x |= uint64(ReverseLUT[byte(v>>0)]) << 56
	x |= uint64(ReverseLUT[byte(v>>8)]) << 48
	x |= uint64(ReverseLUT[byte(v>>16)]) << 40
	x |= uint64(ReverseLUT[byte(v>>24)]) << 32
	x |= uint64(ReverseLUT[byte(v>>32)]) << 24
	x |= uint64(ReverseLUT[byte(v>>40)]) << 16
	x |= uint64(ReverseLUT[byte(v>>48)]) << 8
	x |= uint64(ReverseLUT[byte(v>>56)]) << 0
	return x
}

// ReverseUint64N reverses the lower n bits of v.
func ReverseUint64N(v uint64, n uint) (x uint64) {
	return ReverseUint64(v << (64 - n))
}

// MoveToFront is a data structure that allows for more efficient move-to-front
// transformations. This specific implementation assumes that the alphabet is
// densely packed within 0..255.
type MoveToFront struct {
	dict [256]uint8 // Mapping from indexes to values
	tail int        // Number of tail bytes that are already ordered
}

func (m *MoveToFront) Encode(vals []uint8) {
	copy(m.dict[:], IdentityLUT[:256-m.tail]) // Reset dict to be identity

	var max int
	for i, val := range vals {
		var idx uint8 // Reverse lookup idx in dict
		for di, dv := range m.dict {
			if dv == val {
				idx = uint8(di)
				break
			}
		}
		vals[i] = idx

		max |= int(idx)
		copy(m.dict[1:], m.dict[:idx])
		m.dict[0] = val
	}
	m.tail = 256 - max - 1
}

func (m *MoveToFront) Decode(idxs []uint8) {
	copy(m.dict[:], IdentityLUT[:256-m.tail]) // Reset dict to be identity

	var max int
	for i, idx := range idxs {
		val := m.dict[idx] // Forward lookup val in dict
		idxs[i] = val

		max |= int(idx)
		copy(m.dict[1:], m.dict[:idx])
		m.dict[0] = val
	}
	m.tail = 256 - max - 1
}
