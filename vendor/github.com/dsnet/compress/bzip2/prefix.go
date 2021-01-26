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

const (
	minNumTrees = 2
	maxNumTrees = 6

	maxPrefixBits = 20      // Maximum bit-width of a prefix code
	maxNumSyms    = 256 + 2 // Maximum number of symbols in the alphabet
	numBlockSyms  = 50      // Number of bytes in a block
)

// encSel and decSel are used to handle the prefix encoding for tree selectors.
// The prefix encoding is as follows:
//
//	Code         TreeIdx
//	0        <=> 0
//	10       <=> 1
//	110      <=> 2
//	1110     <=> 3
//	11110    <=> 4
//	111110   <=> 5
//	111111   <=> 6	Invalid tree index, so should fail
//
var encSel, decSel = func() (e prefix.Encoder, d prefix.Decoder) {
	var selCodes [maxNumTrees + 1]prefix.PrefixCode
	for i := range selCodes {
		selCodes[i] = prefix.PrefixCode{Sym: uint32(i), Len: uint32(i + 1)}
	}
	selCodes[maxNumTrees] = prefix.PrefixCode{Sym: maxNumTrees, Len: maxNumTrees}
	prefix.GeneratePrefixes(selCodes[:])
	e.Init(selCodes[:])
	d.Init(selCodes[:])
	return
}()

type prefixReader struct{ prefix.Reader }

func (pr *prefixReader) Init(r io.Reader) {
	pr.Reader.Init(r, true)
}

func (pr *prefixReader) ReadBitsBE64(nb uint) uint64 {
	if nb <= 32 {
		v := uint32(pr.ReadBits(nb))
		return uint64(internal.ReverseUint32N(v, nb))
	}
	v0 := internal.ReverseUint32(uint32(pr.ReadBits(32)))
	v1 := internal.ReverseUint32(uint32(pr.ReadBits(nb - 32)))
	v := uint64(v0)<<32 | uint64(v1)
	return v >> (64 - nb)
}

func (pr *prefixReader) ReadPrefixCodes(codes []prefix.PrefixCodes, trees []prefix.Decoder) {
	for i, pc := range codes {
		clen := int(pr.ReadBitsBE64(5))
		sum := 1 << maxPrefixBits
		for sym := range pc {
			for {
				if clen < 1 || clen > maxPrefixBits {
					panicf(errors.Corrupted, "invalid prefix bit-length: %d", clen)
				}

				b, ok := pr.TryReadBits(1)
				if !ok {
					b = pr.ReadBits(1)
				}
				if b == 0 {
					break
				}

				b, ok = pr.TryReadBits(1)
				if !ok {
					b = pr.ReadBits(1)
				}
				clen -= int(b*2) - 1 // +1 or -1
			}
			pc[sym] = prefix.PrefixCode{Sym: uint32(sym), Len: uint32(clen)}
			sum -= (1 << maxPrefixBits) >> uint(clen)
		}

		if sum == 0 {
			// Fast path, but only handles complete trees.
			if err := prefix.GeneratePrefixes(pc); err != nil {
				errors.Panic(err) // Using complete trees; should never fail
			}
		} else {
			// Slow path, but handles anything.
			pc = handleDegenerateCodes(pc) // Never fails, but may fail later
			codes[i] = pc
		}
		trees[i].Init(pc)
	}
}

type prefixWriter struct{ prefix.Writer }

func (pw *prefixWriter) Init(w io.Writer) {
	pw.Writer.Init(w, true)
}

func (pw *prefixWriter) WriteBitsBE64(v uint64, nb uint) {
	if nb <= 32 {
		v := internal.ReverseUint32N(uint32(v), nb)
		pw.WriteBits(uint(v), nb)
		return
	}
	v <<= (64 - nb)
	v0 := internal.ReverseUint32(uint32(v >> 32))
	v1 := internal.ReverseUint32(uint32(v))
	pw.WriteBits(uint(v0), 32)
	pw.WriteBits(uint(v1), nb-32)
	return
}

func (pw *prefixWriter) WritePrefixCodes(codes []prefix.PrefixCodes, trees []prefix.Encoder) {
	for i, pc := range codes {
		if err := prefix.GeneratePrefixes(pc); err != nil {
			errors.Panic(err) // Using complete trees; should never fail
		}
		trees[i].Init(pc)

		clen := int(pc[0].Len)
		pw.WriteBitsBE64(uint64(clen), 5)
		for _, c := range pc {
			for int(c.Len) < clen {
				pw.WriteBits(3, 2) // 11
				clen--
			}
			for int(c.Len) > clen {
				pw.WriteBits(1, 2) // 10
				clen++
			}
			pw.WriteBits(0, 1)
		}
	}
}

// handleDegenerateCodes converts a degenerate tree into a canonical tree.
//
// For example, when the input is an under-subscribed tree:
//	input:  []PrefixCode{
//		{Sym: 0, Len: 3},
//		{Sym: 1, Len: 4},
//		{Sym: 2, Len: 3},
//	}
//	output: []PrefixCode{
//		{Sym:   0, Len: 3, Val:  0}, //  000
//		{Sym:   1, Len: 4, Val:  2}, // 0010
//		{Sym:   2, Len: 3, Val:  4}, //  100
//		{Sym: 258, Len: 4, Val: 10}, // 1010
//		{Sym: 259, Len: 3, Val:  6}, //  110
//		{Sym: 260, Len: 1, Val:  1}, //    1
//	}
//
// For example, when the input is an over-subscribed tree:
//	input:  []PrefixCode{
//		{Sym: 0, Len: 1},
//		{Sym: 1, Len: 3},
//		{Sym: 2, Len: 4},
//		{Sym: 3, Len: 3},
//		{Sym: 4, Len: 2},
//	}
//	output: []PrefixCode{
//		{Sym: 0, Len: 1, Val: 0}, //   0
//		{Sym: 1, Len: 3, Val: 3}, // 011
//		{Sym: 3, Len: 3, Val: 7}, // 111
//		{Sym: 4, Len: 2, Val: 1}, //  01
//	}
func handleDegenerateCodes(codes prefix.PrefixCodes) prefix.PrefixCodes {
	// Since there is no formal definition for the BZip2 format, there is no
	// specification that says that the code lengths must form a complete
	// prefix tree (IE: it is neither over-subscribed nor under-subscribed).
	// Thus, the original C implementation becomes the reference for how prefix
	// decoding is done in these edge cases. Unfortunately, the C version does
	// not error when an invalid tree is used, but rather allows decoding to
	// continue and only errors if some bit pattern happens to cause an error.
	// Thus, it is possible for an invalid tree to end up decoding an input
	// "properly" so long as invalid bit patterns are not present. In order to
	// replicate this non-specified behavior, we use a ported version of the
	// C code to generate the codes as a valid canonical tree by substituting
	// invalid nodes with invalid symbols.
	//
	// ====================================================
	// This program, "bzip2", the associated library "libbzip2", and all
	// documentation, are copyright (C) 1996-2010 Julian R Seward.  All
	// rights reserved.
	//
	// Redistribution and use in source and binary forms, with or without
	// modification, are permitted provided that the following conditions
	// are met:
	//
	// 1. Redistributions of source code must retain the above copyright
	//    notice, this list of conditions and the following disclaimer.
	//
	// 2. The origin of this software must not be misrepresented; you must
	//    not claim that you wrote the original software.  If you use this
	//    software in a product, an acknowledgment in the product
	//    documentation would be appreciated but is not required.
	//
	// 3. Altered source versions must be plainly marked as such, and must
	//    not be misrepresented as being the original software.
	//
	// 4. The name of the author may not be used to endorse or promote
	//    products derived from this software without specific prior written
	//    permission.
	//
	// THIS SOFTWARE IS PROVIDED BY THE AUTHOR ``AS IS'' AND ANY EXPRESS
	// OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
	// WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
	// ARE DISCLAIMED.  IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY
	// DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
	// DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE
	// GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
	// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY,
	// WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
	// NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
	// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
	//
	// Julian Seward, jseward@bzip.org
	// bzip2/libbzip2 version 1.0.6 of 6 September 2010
	// ====================================================
	var (
		limits [maxPrefixBits + 2]int32
		bases  [maxPrefixBits + 2]int32
		perms  [maxNumSyms]int32

		minLen = uint32(maxPrefixBits)
		maxLen = uint32(0)
	)

	const (
		statusOkay = iota
		statusInvalid
		statusNeedBits
		statusMaxBits
	)

	// createTables is the BZ2_hbCreateDecodeTables function from the C code.
	createTables := func(codes []prefix.PrefixCode) {
		for _, c := range codes {
			if c.Len > maxLen {
				maxLen = c.Len
			}
			if c.Len < minLen {
				minLen = c.Len
			}
		}

		var pp int
		for i := minLen; i <= maxLen; i++ {
			for j, c := range codes {
				if c.Len == i {
					perms[pp] = int32(j)
					pp++
				}
			}
		}

		var vec int32
		for _, c := range codes {
			bases[c.Len+1]++
		}
		for i := 1; i < len(bases); i++ {
			bases[i] += bases[i-1]
		}
		for i := minLen; i <= maxLen; i++ {
			vec += bases[i+1] - bases[i]
			limits[i] = vec - 1
			vec <<= 1
		}
		for i := minLen + 1; i <= maxLen; i++ {
			bases[i] = ((limits[i-1] + 1) << 1) - bases[i]
		}
	}

	// getSymbol is the GET_MTF_VAL macro from the C code.
	getSymbol := func(c prefix.PrefixCode) (uint32, int) {
		v := internal.ReverseUint32(c.Val)
		n := c.Len

		zn := minLen
		if zn > n {
			return 0, statusNeedBits
		}
		zvec := int32(v >> (32 - zn))
		v <<= zn
		for {
			if zn > maxLen {
				return 0, statusMaxBits
			}
			if zvec <= limits[zn] {
				break
			}
			zn++
			if zn > n {
				return 0, statusNeedBits
			}
			zvec = (zvec << 1) | int32(v>>31)
			v <<= 1
		}
		if zvec-bases[zn] < 0 || zvec-bases[zn] >= maxNumSyms {
			return 0, statusInvalid
		}
		return uint32(perms[zvec-bases[zn]]), statusOkay
	}

	// Step 1: Create the prefix trees using the C algorithm.
	createTables(codes)

	// Step 2: Starting with the shortest bit pattern, explore the whole tree.
	// If tree is under-subscribed, the worst-case runtime is O(1<<maxLen).
	// If tree is over-subscribed, the worst-case runtime is O(maxNumSyms).
	var pcodesArr [2 * maxNumSyms]prefix.PrefixCode
	pcodes := pcodesArr[:maxNumSyms]
	var exploreCode func(prefix.PrefixCode) bool
	exploreCode = func(c prefix.PrefixCode) (term bool) {
		sym, status := getSymbol(c)
		switch status {
		case statusOkay:
			// This code is valid, so insert it.
			c.Sym = sym
			pcodes[sym] = c
			term = true
		case statusInvalid:
			// This code is invalid, so insert an invalid symbol.
			c.Sym = uint32(len(pcodes))
			pcodes = append(pcodes, c)
			term = true
		case statusNeedBits:
			// This code is too short, so explore both children.
			c.Len++
			c0, c1 := c, c
			c1.Val |= 1 << (c.Len - 1)

			b0 := exploreCode(c0)
			b1 := exploreCode(c1)
			switch {
			case !b0 && b1:
				c0.Sym = uint32(len(pcodes))
				pcodes = append(pcodes, c0)
			case !b1 && b0:
				c1.Sym = uint32(len(pcodes))
				pcodes = append(pcodes, c1)
			}
			term = b0 || b1
		case statusMaxBits:
			// This code is too long, so report it upstream.
			term = false
		}
		return term // Did this code terminate?
	}
	exploreCode(prefix.PrefixCode{})

	// Step 3: Copy new sparse codes to old output codes.
	codes = codes[:0]
	for _, c := range pcodes {
		if c.Len > 0 {
			codes = append(codes, c)
		}
	}
	return codes
}
