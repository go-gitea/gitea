// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package prefix

import (
	"sort"

	"github.com/dsnet/compress/internal"
)

// The algorithm used to decode variable length codes is based on the lookup
// method in zlib. If the code is less-than-or-equal to maxChunkBits,
// then the symbol can be decoded using a single lookup into the chunks table.
// Otherwise, the links table will be used for a second level lookup.
//
// The chunks slice is keyed by the contents of the bit buffer ANDed with
// the chunkMask to avoid a out-of-bounds lookup. The value of chunks is a tuple
// that is decoded as follow:
//
//	var length = chunks[bitBuffer&chunkMask] & countMask
//	var symbol = chunks[bitBuffer&chunkMask] >> countBits
//
// If the decoded length is larger than chunkBits, then an overflow link table
// must be used for further decoding. In this case, the symbol is actually the
// index into the links tables. The second-level links table returned is
// processed in the same way as the chunks table.
//
//	if length > chunkBits {
//		var index = symbol // Previous symbol is index into links tables
//		length = links[index][bitBuffer>>chunkBits & linkMask] & countMask
//		symbol = links[index][bitBuffer>>chunkBits & linkMask] >> countBits
//	}
//
// See the following:
//	http://www.gzip.org/algorithm.txt

type Decoder struct {
	chunks    []uint32   // First-level lookup map
	links     [][]uint32 // Second-level lookup map
	chunkMask uint32     // Mask the length of the chunks table
	linkMask  uint32     // Mask the length of the link table
	chunkBits uint32     // Bit-length of the chunks table

	MinBits uint32 // The minimum number of bits to safely make progress
	NumSyms uint32 // Number of symbols
}

// Init initializes Decoder according to the codes provided.
func (pd *Decoder) Init(codes PrefixCodes) {
	// Handle special case trees.
	if len(codes) <= 1 {
		switch {
		case len(codes) == 0: // Empty tree (should error if used later)
			*pd = Decoder{chunks: pd.chunks[:0], links: pd.links[:0], NumSyms: 0}
		case len(codes) == 1 && codes[0].Len == 0: // Single code tree (bit-length of zero)
			pd.chunks = append(pd.chunks[:0], codes[0].Sym<<countBits|0)
			*pd = Decoder{chunks: pd.chunks[:1], links: pd.links[:0], NumSyms: 1}
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

	var minBits, maxBits uint32 = valueBits, 0
	for _, c := range codes {
		if minBits > c.Len {
			minBits = c.Len
		}
		if maxBits < c.Len {
			maxBits = c.Len
		}
	}

	// Allocate chunks table as needed.
	const maxChunkBits = 9 // This can be tuned for better performance
	pd.NumSyms = uint32(len(codes))
	pd.MinBits = minBits
	pd.chunkBits = maxBits
	if pd.chunkBits > maxChunkBits {
		pd.chunkBits = maxChunkBits
	}
	numChunks := 1 << pd.chunkBits
	pd.chunks = allocUint32s(pd.chunks, numChunks)
	pd.chunkMask = uint32(numChunks - 1)

	// Allocate links tables as needed.
	pd.links = pd.links[:0]
	pd.linkMask = 0
	if pd.chunkBits < maxBits {
		numLinks := 1 << (maxBits - pd.chunkBits)
		pd.linkMask = uint32(numLinks - 1)

		var linkIdx uint32
		for i := range pd.chunks {
			pd.chunks[i] = 0 // Logic below relies on zero value as uninitialized
		}
		for _, c := range codes {
			if c.Len > pd.chunkBits && pd.chunks[c.Val&pd.chunkMask] == 0 {
				pd.chunks[c.Val&pd.chunkMask] = (linkIdx << countBits) | (pd.chunkBits + 1)
				linkIdx++
			}
		}

		pd.links = extendSliceUint32s(pd.links, int(linkIdx))
		linksFlat := allocUint32s(pd.links[0], numLinks*int(linkIdx))
		for i, j := 0, 0; i < len(pd.links); i, j = i+1, j+numLinks {
			pd.links[i] = linksFlat[j : j+numLinks]
		}
	}

	// Fill out chunks and links tables with values.
	for _, c := range codes {
		chunk := c.Sym<<countBits | c.Len
		if c.Len <= pd.chunkBits {
			skip := 1 << uint(c.Len)
			for j := int(c.Val); j < len(pd.chunks); j += skip {
				pd.chunks[j] = chunk
			}
		} else {
			linkIdx := pd.chunks[c.Val&pd.chunkMask] >> countBits
			links := pd.links[linkIdx]
			skip := 1 << uint(c.Len-pd.chunkBits)
			for j := int(c.Val >> pd.chunkBits); j < len(links); j += skip {
				links[j] = chunk
			}
		}
	}
}
