// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build gofuzz

// This file exists to export internal implementation details for fuzz testing.

package bzip2

func ForwardBWT(buf []byte) (ptr int) {
	var bwt burrowsWheelerTransform
	return bwt.Encode(buf)
}

func ReverseBWT(buf []byte, ptr int) {
	var bwt burrowsWheelerTransform
	bwt.Decode(buf, ptr)
}

type fuzzReader struct {
	Checksums Checksums
}

// updateChecksum updates Checksums.
//
// If a valid pos is provided, it appends the (pos, val) pair to the slice.
// Otherwise, it will update the last record with the new value.
func (fr *fuzzReader) updateChecksum(pos int64, val uint32) {
	if pos >= 0 {
		fr.Checksums = append(fr.Checksums, Checksum{pos, val})
	} else {
		fr.Checksums[len(fr.Checksums)-1].Value = val
	}
}

type Checksum struct {
	Offset int64  // Bit offset of the checksum
	Value  uint32 // Checksum value
}

type Checksums []Checksum

// Apply overwrites all checksum fields in d with the ones in cs.
func (cs Checksums) Apply(d []byte) []byte {
	d = append([]byte(nil), d...)
	for _, c := range cs {
		setU32(d, c.Offset, c.Value)
	}
	return d
}

func setU32(d []byte, pos int64, val uint32) {
	for i := uint(0); i < 32; i++ {
		bpos := uint64(pos) + uint64(i)
		d[bpos/8] &= ^byte(1 << (7 - bpos%8))
		d[bpos/8] |= byte(val>>(31-i)) << (7 - bpos%8)
	}
}

// Verify checks that all checksum fields in d matches those in cs.
func (cs Checksums) Verify(d []byte) bool {
	for _, c := range cs {
		if getU32(d, c.Offset) != c.Value {
			return false
		}
	}
	return true
}

func getU32(d []byte, pos int64) (val uint32) {
	for i := uint(0); i < 32; i++ {
		bpos := uint64(pos) + uint64(i)
		val |= (uint32(d[bpos/8] >> (7 - bpos%8))) << (31 - i)
	}
	return val
}
