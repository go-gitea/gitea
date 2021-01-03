// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"io"

	"github.com/dsnet/compress/internal"
	"github.com/dsnet/compress/internal/errors"
	"github.com/dsnet/compress/internal/prefix"
)

type Writer struct {
	InputOffset  int64 // Total number of bytes issued to Write
	OutputOffset int64 // Total number of bytes written to underlying io.Writer

	wr     prefixWriter
	err    error
	level  int    // The current compression level
	wrHdr  bool   // Have we written the stream header?
	blkCRC uint32 // CRC-32 IEEE of each block
	endCRC uint32 // Checksum of all blocks using bzip2's custom method

	crc crc
	rle runLengthEncoding
	bwt burrowsWheelerTransform
	mtf moveToFront

	// These fields are allocated with Writer and re-used later.
	buf         []byte
	treeSels    []uint8
	treeSelsMTF []uint8
	codes2D     [maxNumTrees][maxNumSyms]prefix.PrefixCode
	codes1D     [maxNumTrees]prefix.PrefixCodes
	trees1D     [maxNumTrees]prefix.Encoder
}

type WriterConfig struct {
	Level int

	_ struct{} // Blank field to prevent unkeyed struct literals
}

func NewWriter(w io.Writer, conf *WriterConfig) (*Writer, error) {
	var lvl int
	if conf != nil {
		lvl = conf.Level
	}
	if lvl == 0 {
		lvl = DefaultCompression
	}
	if lvl < BestSpeed || lvl > BestCompression {
		return nil, errorf(errors.Invalid, "compression level: %d", lvl)
	}
	zw := new(Writer)
	zw.level = lvl
	zw.Reset(w)
	return zw, nil
}

func (zw *Writer) Reset(w io.Writer) error {
	*zw = Writer{
		wr:    zw.wr,
		level: zw.level,

		rle: zw.rle,
		bwt: zw.bwt,
		mtf: zw.mtf,

		buf:         zw.buf,
		treeSels:    zw.treeSels,
		treeSelsMTF: zw.treeSelsMTF,
		trees1D:     zw.trees1D,
	}
	zw.wr.Init(w)
	if len(zw.buf) != zw.level*blockSize {
		zw.buf = make([]byte, zw.level*blockSize)
	}
	zw.rle.Init(zw.buf)
	return nil
}

func (zw *Writer) Write(buf []byte) (int, error) {
	if zw.err != nil {
		return 0, zw.err
	}

	cnt := len(buf)
	for {
		wrCnt, err := zw.rle.Write(buf)
		if err != rleDone && zw.err == nil {
			zw.err = err
		}
		zw.crc.update(buf[:wrCnt])
		buf = buf[wrCnt:]
		if len(buf) == 0 {
			zw.InputOffset += int64(cnt)
			return cnt, nil
		}
		if zw.err = zw.flush(); zw.err != nil {
			return 0, zw.err
		}
	}
}

func (zw *Writer) flush() error {
	vals := zw.rle.Bytes()
	if len(vals) == 0 {
		return nil
	}
	zw.wr.Offset = zw.OutputOffset
	func() {
		defer errors.Recover(&zw.err)
		if !zw.wrHdr {
			// Write stream header.
			zw.wr.WriteBitsBE64(hdrMagic, 16)
			zw.wr.WriteBitsBE64('h', 8)
			zw.wr.WriteBitsBE64(uint64('0'+zw.level), 8)
			zw.wrHdr = true
		}
		zw.encodeBlock(vals)
	}()
	var err error
	if zw.OutputOffset, err = zw.wr.Flush(); zw.err == nil {
		zw.err = err
	}
	if zw.err != nil {
		zw.err = errWrap(zw.err, errors.Internal)
		return zw.err
	}
	zw.endCRC = (zw.endCRC<<1 | zw.endCRC>>31) ^ zw.blkCRC
	zw.blkCRC = 0
	zw.rle.Init(zw.buf)
	return nil
}

func (zw *Writer) Close() error {
	if zw.err == errClosed {
		return nil
	}

	// Flush RLE buffer if there is left-over data.
	if zw.err = zw.flush(); zw.err != nil {
		return zw.err
	}

	// Write stream footer.
	zw.wr.Offset = zw.OutputOffset
	func() {
		defer errors.Recover(&zw.err)
		if !zw.wrHdr {
			// Write stream header.
			zw.wr.WriteBitsBE64(hdrMagic, 16)
			zw.wr.WriteBitsBE64('h', 8)
			zw.wr.WriteBitsBE64(uint64('0'+zw.level), 8)
			zw.wrHdr = true
		}
		zw.wr.WriteBitsBE64(endMagic, 48)
		zw.wr.WriteBitsBE64(uint64(zw.endCRC), 32)
		zw.wr.WritePads(0)
	}()
	var err error
	if zw.OutputOffset, err = zw.wr.Flush(); zw.err == nil {
		zw.err = err
	}
	if zw.err != nil {
		zw.err = errWrap(zw.err, errors.Internal)
		return zw.err
	}

	zw.err = errClosed
	return nil
}

func (zw *Writer) encodeBlock(buf []byte) {
	zw.blkCRC = zw.crc.val
	zw.wr.WriteBitsBE64(blkMagic, 48)
	zw.wr.WriteBitsBE64(uint64(zw.blkCRC), 32)
	zw.wr.WriteBitsBE64(0, 1)
	zw.crc.val = 0

	// Step 1: Burrows-Wheeler transformation.
	ptr := zw.bwt.Encode(buf)
	zw.wr.WriteBitsBE64(uint64(ptr), 24)

	// Step 2: Move-to-front transform and run-length encoding.
	var dictMap [256]bool
	for _, c := range buf {
		dictMap[c] = true
	}

	var dictArr [256]uint8
	var bmapLo [16]uint16
	dict := dictArr[:0]
	bmapHi := uint16(0)
	for i, b := range dictMap {
		if b {
			c := uint8(i)
			dict = append(dict, c)
			bmapHi |= 1 << (c >> 4)
			bmapLo[c>>4] |= 1 << (c & 0xf)
		}
	}

	zw.wr.WriteBits(uint(bmapHi), 16)
	for _, m := range bmapLo {
		if m > 0 {
			zw.wr.WriteBits(uint(m), 16)
		}
	}

	zw.mtf.Init(dict, len(buf))
	syms := zw.mtf.Encode(buf)

	// Step 3: Prefix encoding.
	zw.encodePrefix(syms, len(dict))
}

func (zw *Writer) encodePrefix(syms []uint16, numSyms int) {
	numSyms += 2 // Remove 0 symbol, add RUNA, RUNB, and EOB symbols
	if numSyms < 3 {
		panicf(errors.Internal, "unable to encode EOB marker")
	}
	syms = append(syms, uint16(numSyms-1)) // EOB marker

	// Compute number of prefix trees needed.
	numTrees := maxNumTrees
	for i, lim := range []int{200, 600, 1200, 2400} {
		if len(syms) < lim {
			numTrees = minNumTrees + i
			break
		}
	}

	// Compute number of block selectors.
	numSels := (len(syms) + numBlockSyms - 1) / numBlockSyms
	if cap(zw.treeSels) < numSels {
		zw.treeSels = make([]uint8, numSels)
	}
	treeSels := zw.treeSels[:numSels]
	for i := range treeSels {
		treeSels[i] = uint8(i % numTrees)
	}

	// Initialize prefix codes.
	for i := range zw.codes2D[:numTrees] {
		pc := zw.codes2D[i][:numSyms]
		for j := range pc {
			pc[j] = prefix.PrefixCode{Sym: uint32(j)}
		}
		zw.codes1D[i] = pc
	}

	// First cut at assigning prefix trees to each group.
	var codes prefix.PrefixCodes
	var blkLen, selIdx int
	for _, sym := range syms {
		if blkLen == 0 {
			blkLen = numBlockSyms
			codes = zw.codes2D[treeSels[selIdx]][:numSyms]
			selIdx++
		}
		blkLen--
		codes[sym].Cnt++
	}

	// TODO(dsnet): Use K-means to cluster groups to each prefix tree.

	// Generate lengths and prefixes based on symbol frequencies.
	for i := range zw.trees1D[:numTrees] {
		pc := prefix.PrefixCodes(zw.codes2D[i][:numSyms])
		pc.SortByCount()
		if err := prefix.GenerateLengths(pc, maxPrefixBits); err != nil {
			errors.Panic(err)
		}
		pc.SortBySymbol()
	}

	// Write out information about the trees and tree selectors.
	var mtf internal.MoveToFront
	zw.wr.WriteBitsBE64(uint64(numTrees), 3)
	zw.wr.WriteBitsBE64(uint64(numSels), 15)
	zw.treeSelsMTF = append(zw.treeSelsMTF[:0], treeSels...)
	mtf.Encode(zw.treeSelsMTF)
	for _, sym := range zw.treeSelsMTF {
		zw.wr.WriteSymbol(uint(sym), &encSel)
	}
	zw.wr.WritePrefixCodes(zw.codes1D[:numTrees], zw.trees1D[:numTrees])

	// Write out prefix encoded symbols of compressed data.
	var tree *prefix.Encoder
	blkLen, selIdx = 0, 0
	for _, sym := range syms {
		if blkLen == 0 {
			blkLen = numBlockSyms
			tree = &zw.trees1D[treeSels[selIdx]]
			selIdx++
		}
		blkLen--
		ok := zw.wr.TryWriteSymbol(uint(sym), tree)
		if !ok {
			zw.wr.WriteSymbol(uint(sym), tree)
		}
	}
}
