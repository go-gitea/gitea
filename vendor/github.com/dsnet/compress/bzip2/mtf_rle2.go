// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import "github.com/dsnet/compress/internal/errors"

// moveToFront implements both the MTF and RLE stages of bzip2 at the same time.
// Any runs of zeros in the encoded output will be replaced by a sequence of
// RUNA and RUNB symbols are encode the length of the run.
//
// The RLE encoding used can actually be encoded to and decoded from using
// normal two's complement arithmetic. The methodology for doing so is below.
//
// Assuming the following:
//	num: The value being encoded by RLE encoding.
//	run: A sequence of RUNA and RUNB symbols represented as a binary integer,
//	where RUNA is the 0 bit, RUNB is the 1 bit, and least-significant RUN
//	symbols are at the least-significant bit positions.
//	cnt: The number of RUNA and RUNB symbols.
//
// Then the RLE encoding used by bzip2 has this mathematical property:
//	num+1 == (1<<cnt) | run
type moveToFront struct {
	dictBuf [256]uint8
	dictLen int

	vals    []byte
	syms    []uint16
	blkSize int
}

func (mtf *moveToFront) Init(dict []uint8, blkSize int) {
	if len(dict) > len(mtf.dictBuf) {
		panicf(errors.Internal, "alphabet too large")
	}
	copy(mtf.dictBuf[:], dict)
	mtf.dictLen = len(dict)
	mtf.blkSize = blkSize
}

func (mtf *moveToFront) Encode(vals []byte) (syms []uint16) {
	dict := mtf.dictBuf[:mtf.dictLen]
	syms = mtf.syms[:0]

	if len(vals) > mtf.blkSize {
		panicf(errors.Internal, "exceeded block size")
	}

	var lastNum uint32
	for _, val := range vals {
		// Normal move-to-front transform.
		var idx uint8 // Reverse lookup idx in dict
		for di, dv := range dict {
			if dv == val {
				idx = uint8(di)
				break
			}
		}
		copy(dict[1:], dict[:idx])
		dict[0] = val

		// Run-length encoding augmentation.
		if idx == 0 {
			lastNum++
			continue
		}
		if lastNum > 0 {
			for rc := lastNum + 1; rc != 1; rc >>= 1 {
				syms = append(syms, uint16(rc&1))
			}
			lastNum = 0
		}
		syms = append(syms, uint16(idx)+1)
	}
	if lastNum > 0 {
		for rc := lastNum + 1; rc != 1; rc >>= 1 {
			syms = append(syms, uint16(rc&1))
		}
	}
	mtf.syms = syms
	return syms
}

func (mtf *moveToFront) Decode(syms []uint16) (vals []byte) {
	dict := mtf.dictBuf[:mtf.dictLen]
	vals = mtf.vals[:0]

	var lastCnt uint
	var lastRun uint32
	for _, sym := range syms {
		// Run-length encoding augmentation.
		if sym < 2 {
			lastRun |= uint32(sym) << lastCnt
			lastCnt++
			continue
		}
		if lastCnt > 0 {
			cnt := int((1<<lastCnt)|lastRun) - 1
			if len(vals)+cnt > mtf.blkSize || lastCnt > 24 {
				panicf(errors.Corrupted, "run-length decoding exceeded block size")
			}
			for i := cnt; i > 0; i-- {
				vals = append(vals, dict[0])
			}
			lastCnt, lastRun = 0, 0
		}

		// Normal move-to-front transform.
		val := dict[sym-1] // Forward lookup val in dict
		copy(dict[1:], dict[:sym-1])
		dict[0] = val

		if len(vals) >= mtf.blkSize {
			panicf(errors.Corrupted, "run-length decoding exceeded block size")
		}
		vals = append(vals, val)
	}
	if lastCnt > 0 {
		cnt := int((1<<lastCnt)|lastRun) - 1
		if len(vals)+cnt > mtf.blkSize || lastCnt > 24 {
			panicf(errors.Corrupted, "run-length decoding exceeded block size")
		}
		for i := cnt; i > 0; i-- {
			vals = append(vals, dict[0])
		}
	}
	mtf.vals = vals
	return vals
}
