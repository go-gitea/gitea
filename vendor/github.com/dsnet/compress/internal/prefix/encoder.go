// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package prefix

import (
	"sort"

	"github.com/dsnet/compress/internal"
)

type Encoder struct {
	chunks    []uint32 // First-level lookup map
	chunkMask uint32   // Mask the length of the chunks table

	NumSyms uint32 // Number of symbols
}

// Init initializes Encoder according to the codes provided.
func (pe *Encoder) Init(codes PrefixCodes) {
	// Handle special case trees.
	if len(codes) <= 1 {
		switch {
		case len(codes) == 0: // Empty tree (should error if used later)
			*pe = Encoder{chunks: pe.chunks[:0], NumSyms: 0}
		case len(codes) == 1 && codes[0].Len == 0: // Single code tree (bit-length of zero)
			pe.chunks = append(pe.chunks[:0], codes[0].Val<<countBits|0)
			*pe = Encoder{chunks: pe.chunks[:1], NumSyms: 1}
		default:
			panic("invalid codes")
		}
		return
	}
	if internal.Debug && !sort.IsSorted(prefixCodesBySymbol(codes)) {
		panic("input codes is not sorted")
	}
	if internal.Debug && !(codes.checkLengths() && codes.checkPrefixes()) {
		panic("detected incomplete or overlapping codes")
	}

	// Enough chunks to contain all the symbols.
	numChunks := 1
	for n := len(codes) - 1; n > 0; n >>= 1 {
		numChunks <<= 1
	}
	pe.NumSyms = uint32(len(codes))

retry:
	// Allocate and reset chunks.
	pe.chunks = allocUint32s(pe.chunks, numChunks)
	pe.chunkMask = uint32(numChunks - 1)
	for i := range pe.chunks {
		pe.chunks[i] = 0 // Logic below relies on zero value as uninitialized
	}

	// Insert each symbol, checking that there are no conflicts.
	for _, c := range codes {
		if pe.chunks[c.Sym&pe.chunkMask] > 0 {
			// Collision found our "hash" table, so grow and try again.
			numChunks <<= 1
			goto retry
		}
		pe.chunks[c.Sym&pe.chunkMask] = c.Val<<countBits | c.Len
	}
}
