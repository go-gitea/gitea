// Copyright 2019+ Klaus Post. All rights reserved.
// License information can be found in the LICENSE file.
// Based on work by Yann Collet, released under BSD License.

package zstd

import (
	"fmt"
	"math/bits"

	"github.com/klauspost/compress/zstd/internal/xxhash"
)

const (
	tableBits      = 15             // Bits used in the table
	tableSize      = 1 << tableBits // Size of the table
	tableMask      = tableSize - 1  // Mask for table indices. Redundant, but can eliminate bounds checks.
	maxMatchLength = 131074
)

type tableEntry struct {
	val    uint32
	offset int32
}

type fastEncoder struct {
	o encParams
	// cur is the offset at the start of hist
	cur int32
	// maximum offset. Should be at least 2x block size.
	maxMatchOff int32
	hist        []byte
	crc         *xxhash.Digest
	table       [tableSize]tableEntry
	tmp         [8]byte
	blk         *blockEnc
}

// CRC returns the underlying CRC writer.
func (e *fastEncoder) CRC() *xxhash.Digest {
	return e.crc
}

// AppendCRC will append the CRC to the destination slice and return it.
func (e *fastEncoder) AppendCRC(dst []byte) []byte {
	crc := e.crc.Sum(e.tmp[:0])
	dst = append(dst, crc[7], crc[6], crc[5], crc[4])
	return dst
}

// WindowSize returns the window size of the encoder,
// or a window size small enough to contain the input size, if > 0.
func (e *fastEncoder) WindowSize(size int) int32 {
	if size > 0 && size < int(e.maxMatchOff) {
		b := int32(1) << uint(bits.Len(uint(size)))
		// Keep minimum window.
		if b < 1024 {
			b = 1024
		}
		return b
	}
	return e.maxMatchOff
}

// Block returns the current block.
func (e *fastEncoder) Block() *blockEnc {
	return e.blk
}

// Encode mimmics functionality in zstd_fast.c
func (e *fastEncoder) Encode(blk *blockEnc, src []byte) {
	const (
		inputMargin            = 8
		minNonLiteralBlockSize = 1 + 1 + inputMargin
	)

	// Protect against e.cur wraparound.
	for e.cur >= bufferReset {
		if len(e.hist) == 0 {
			for i := range e.table[:] {
				e.table[i] = tableEntry{}
			}
			e.cur = e.maxMatchOff
			break
		}
		// Shift down everything in the table that isn't already too far away.
		minOff := e.cur + int32(len(e.hist)) - e.maxMatchOff
		for i := range e.table[:] {
			v := e.table[i].offset
			if v < minOff {
				v = 0
			} else {
				v = v - e.cur + e.maxMatchOff
			}
			e.table[i].offset = v
		}
		e.cur = e.maxMatchOff
		break
	}

	s := e.addBlock(src)
	blk.size = len(src)
	if len(src) < minNonLiteralBlockSize {
		blk.extraLits = len(src)
		blk.literals = blk.literals[:len(src)]
		copy(blk.literals, src)
		return
	}

	// Override src
	src = e.hist
	sLimit := int32(len(src)) - inputMargin
	// stepSize is the number of bytes to skip on every main loop iteration.
	// It should be >= 2.
	stepSize := int32(e.o.targetLength)
	if stepSize == 0 {
		stepSize++
	}
	stepSize++

	// TEMPLATE
	const hashLog = tableBits
	// seems global, but would be nice to tweak.
	const kSearchStrength = 8

	// nextEmit is where in src the next emitLiteral should start from.
	nextEmit := s
	cv := load6432(src, s)

	// Relative offsets
	offset1 := int32(blk.recentOffsets[0])
	offset2 := int32(blk.recentOffsets[1])

	addLiterals := func(s *seq, until int32) {
		if until == nextEmit {
			return
		}
		blk.literals = append(blk.literals, src[nextEmit:until]...)
		s.litLen = uint32(until - nextEmit)
	}
	if debug {
		println("recent offsets:", blk.recentOffsets)
	}

encodeLoop:
	for {
		// t will contain the match offset when we find one.
		// When existing the search loop, we have already checked 4 bytes.
		var t int32

		// We will not use repeat offsets across blocks.
		// By not using them for the first 3 matches
		canRepeat := len(blk.sequences) > 2

		for {
			if debugAsserts && canRepeat && offset1 == 0 {
				panic("offset0 was 0")
			}

			nextHash := hash6(cv, hashLog)
			nextHash2 := hash6(cv>>8, hashLog)
			candidate := e.table[nextHash]
			candidate2 := e.table[nextHash2]
			repIndex := s - offset1 + 2

			e.table[nextHash] = tableEntry{offset: s + e.cur, val: uint32(cv)}
			e.table[nextHash2] = tableEntry{offset: s + e.cur + 1, val: uint32(cv >> 8)}

			if canRepeat && repIndex >= 0 && load3232(src, repIndex) == uint32(cv>>16) {
				// Consider history as well.
				var seq seq
				lenght := 4 + e.matchlen(s+6, repIndex+4, src)

				seq.matchLen = uint32(lenght - zstdMinMatch)

				// We might be able to match backwards.
				// Extend as long as we can.
				start := s + 2
				// We end the search early, so we don't risk 0 literals
				// and have to do special offset treatment.
				startLimit := nextEmit + 1

				sMin := s - e.maxMatchOff
				if sMin < 0 {
					sMin = 0
				}
				for repIndex > sMin && start > startLimit && src[repIndex-1] == src[start-1] && seq.matchLen < maxMatchLength-zstdMinMatch {
					repIndex--
					start--
					seq.matchLen++
				}
				addLiterals(&seq, start)

				// rep 0
				seq.offset = 1
				if debugSequences {
					println("repeat sequence", seq, "next s:", s)
				}
				blk.sequences = append(blk.sequences, seq)
				s += lenght + 2
				nextEmit = s
				if s >= sLimit {
					if debug {
						println("repeat ended", s, lenght)

					}
					break encodeLoop
				}
				cv = load6432(src, s)
				continue
			}
			coffset0 := s - (candidate.offset - e.cur)
			coffset1 := s - (candidate2.offset - e.cur) + 1
			if coffset0 < e.maxMatchOff && uint32(cv) == candidate.val {
				// found a regular match
				t = candidate.offset - e.cur
				if debugAsserts && s <= t {
					panic(fmt.Sprintf("s (%d) <= t (%d)", s, t))
				}
				if debugAsserts && s-t > e.maxMatchOff {
					panic("s - t >e.maxMatchOff")
				}
				break
			}

			if coffset1 < e.maxMatchOff && uint32(cv>>8) == candidate2.val {
				// found a regular match
				t = candidate2.offset - e.cur
				s++
				if debugAsserts && s <= t {
					panic(fmt.Sprintf("s (%d) <= t (%d)", s, t))
				}
				if debugAsserts && s-t > e.maxMatchOff {
					panic("s - t >e.maxMatchOff")
				}
				if debugAsserts && t < 0 {
					panic("t<0")
				}
				break
			}
			s += stepSize + ((s - nextEmit) >> (kSearchStrength - 1))
			if s >= sLimit {
				break encodeLoop
			}
			cv = load6432(src, s)
		}
		// A 4-byte match has been found. We'll later see if more than 4 bytes.
		offset2 = offset1
		offset1 = s - t

		if debugAsserts && s <= t {
			panic(fmt.Sprintf("s (%d) <= t (%d)", s, t))
		}

		if debugAsserts && canRepeat && int(offset1) > len(src) {
			panic("invalid offset")
		}

		// Extend the 4-byte match as long as possible.
		l := e.matchlen(s+4, t+4, src) + 4

		// Extend backwards
		tMin := s - e.maxMatchOff
		if tMin < 0 {
			tMin = 0
		}
		for t > tMin && s > nextEmit && src[t-1] == src[s-1] && l < maxMatchLength {
			s--
			t--
			l++
		}

		// Write our sequence.
		var seq seq
		seq.litLen = uint32(s - nextEmit)
		seq.matchLen = uint32(l - zstdMinMatch)
		if seq.litLen > 0 {
			blk.literals = append(blk.literals, src[nextEmit:s]...)
		}
		// Don't use repeat offsets
		seq.offset = uint32(s-t) + 3
		s += l
		if debugSequences {
			println("sequence", seq, "next s:", s)
		}
		blk.sequences = append(blk.sequences, seq)
		nextEmit = s
		if s >= sLimit {
			break encodeLoop
		}
		cv = load6432(src, s)

		// Check offset 2
		if o2 := s - offset2; canRepeat && load3232(src, o2) == uint32(cv) {
			// We have at least 4 byte match.
			// No need to check backwards. We come straight from a match
			l := 4 + e.matchlen(s+4, o2+4, src)

			// Store this, since we have it.
			nextHash := hash6(cv, hashLog)
			e.table[nextHash] = tableEntry{offset: s + e.cur, val: uint32(cv)}
			seq.matchLen = uint32(l) - zstdMinMatch
			seq.litLen = 0
			// Since litlen is always 0, this is offset 1.
			seq.offset = 1
			s += l
			nextEmit = s
			if debugSequences {
				println("sequence", seq, "next s:", s)
			}
			blk.sequences = append(blk.sequences, seq)

			// Swap offset 1 and 2.
			offset1, offset2 = offset2, offset1
			if s >= sLimit {
				break encodeLoop
			}
			// Prepare next loop.
			cv = load6432(src, s)
		}
	}

	if int(nextEmit) < len(src) {
		blk.literals = append(blk.literals, src[nextEmit:]...)
		blk.extraLits = len(src) - int(nextEmit)
	}
	blk.recentOffsets[0] = uint32(offset1)
	blk.recentOffsets[1] = uint32(offset2)
	if debug {
		println("returning, recent offsets:", blk.recentOffsets, "extra literals:", blk.extraLits)
	}
}

// EncodeNoHist will encode a block with no history and no following blocks.
// Most notable difference is that src will not be copied for history and
// we do not need to check for max match length.
func (e *fastEncoder) EncodeNoHist(blk *blockEnc, src []byte) {
	const (
		inputMargin            = 8
		minNonLiteralBlockSize = 1 + 1 + inputMargin
	)
	if debug {
		if len(src) > maxBlockSize {
			panic("src too big")
		}
	}
	// Protect against e.cur wraparound.
	if e.cur >= bufferReset {
		for i := range e.table[:] {
			e.table[i] = tableEntry{}
		}
		e.cur = e.maxMatchOff
	}

	s := int32(0)
	blk.size = len(src)
	if len(src) < minNonLiteralBlockSize {
		blk.extraLits = len(src)
		blk.literals = blk.literals[:len(src)]
		copy(blk.literals, src)
		return
	}

	sLimit := int32(len(src)) - inputMargin
	// stepSize is the number of bytes to skip on every main loop iteration.
	// It should be >= 2.
	const stepSize = 2

	// TEMPLATE
	const hashLog = tableBits
	// seems global, but would be nice to tweak.
	const kSearchStrength = 8

	// nextEmit is where in src the next emitLiteral should start from.
	nextEmit := s
	cv := load6432(src, s)

	// Relative offsets
	offset1 := int32(blk.recentOffsets[0])
	offset2 := int32(blk.recentOffsets[1])

	addLiterals := func(s *seq, until int32) {
		if until == nextEmit {
			return
		}
		blk.literals = append(blk.literals, src[nextEmit:until]...)
		s.litLen = uint32(until - nextEmit)
	}
	if debug {
		println("recent offsets:", blk.recentOffsets)
	}

encodeLoop:
	for {
		// t will contain the match offset when we find one.
		// When existing the search loop, we have already checked 4 bytes.
		var t int32

		// We will not use repeat offsets across blocks.
		// By not using them for the first 3 matches

		for {
			nextHash := hash6(cv, hashLog)
			nextHash2 := hash6(cv>>8, hashLog)
			candidate := e.table[nextHash]
			candidate2 := e.table[nextHash2]
			repIndex := s - offset1 + 2

			e.table[nextHash] = tableEntry{offset: s + e.cur, val: uint32(cv)}
			e.table[nextHash2] = tableEntry{offset: s + e.cur + 1, val: uint32(cv >> 8)}

			if len(blk.sequences) > 2 && load3232(src, repIndex) == uint32(cv>>16) {
				// Consider history as well.
				var seq seq
				// lenght := 4 + e.matchlen(s+6, repIndex+4, src)
				lenght := 4 + int32(matchLen(src[s+6:], src[repIndex+4:]))

				seq.matchLen = uint32(lenght - zstdMinMatch)

				// We might be able to match backwards.
				// Extend as long as we can.
				start := s + 2
				// We end the search early, so we don't risk 0 literals
				// and have to do special offset treatment.
				startLimit := nextEmit + 1

				sMin := s - e.maxMatchOff
				if sMin < 0 {
					sMin = 0
				}
				for repIndex > sMin && start > startLimit && src[repIndex-1] == src[start-1] {
					repIndex--
					start--
					seq.matchLen++
				}
				addLiterals(&seq, start)

				// rep 0
				seq.offset = 1
				if debugSequences {
					println("repeat sequence", seq, "next s:", s)
				}
				blk.sequences = append(blk.sequences, seq)
				s += lenght + 2
				nextEmit = s
				if s >= sLimit {
					if debug {
						println("repeat ended", s, lenght)

					}
					break encodeLoop
				}
				cv = load6432(src, s)
				continue
			}
			coffset0 := s - (candidate.offset - e.cur)
			coffset1 := s - (candidate2.offset - e.cur) + 1
			if coffset0 < e.maxMatchOff && uint32(cv) == candidate.val {
				// found a regular match
				t = candidate.offset - e.cur
				if debugAsserts && s <= t {
					panic(fmt.Sprintf("s (%d) <= t (%d)", s, t))
				}
				if debugAsserts && s-t > e.maxMatchOff {
					panic("s - t >e.maxMatchOff")
				}
				break
			}

			if coffset1 < e.maxMatchOff && uint32(cv>>8) == candidate2.val {
				// found a regular match
				t = candidate2.offset - e.cur
				s++
				if debugAsserts && s <= t {
					panic(fmt.Sprintf("s (%d) <= t (%d)", s, t))
				}
				if debugAsserts && s-t > e.maxMatchOff {
					panic("s - t >e.maxMatchOff")
				}
				if debugAsserts && t < 0 {
					panic("t<0")
				}
				break
			}
			s += stepSize + ((s - nextEmit) >> (kSearchStrength - 1))
			if s >= sLimit {
				break encodeLoop
			}
			cv = load6432(src, s)
		}
		// A 4-byte match has been found. We'll later see if more than 4 bytes.
		offset2 = offset1
		offset1 = s - t

		if debugAsserts && s <= t {
			panic(fmt.Sprintf("s (%d) <= t (%d)", s, t))
		}

		// Extend the 4-byte match as long as possible.
		//l := e.matchlenNoHist(s+4, t+4, src) + 4
		l := int32(matchLen(src[s+4:], src[t+4:])) + 4

		// Extend backwards
		tMin := s - e.maxMatchOff
		if tMin < 0 {
			tMin = 0
		}
		for t > tMin && s > nextEmit && src[t-1] == src[s-1] {
			s--
			t--
			l++
		}

		// Write our sequence.
		var seq seq
		seq.litLen = uint32(s - nextEmit)
		seq.matchLen = uint32(l - zstdMinMatch)
		if seq.litLen > 0 {
			blk.literals = append(blk.literals, src[nextEmit:s]...)
		}
		// Don't use repeat offsets
		seq.offset = uint32(s-t) + 3
		s += l
		if debugSequences {
			println("sequence", seq, "next s:", s)
		}
		blk.sequences = append(blk.sequences, seq)
		nextEmit = s
		if s >= sLimit {
			break encodeLoop
		}
		cv = load6432(src, s)

		// Check offset 2
		if o2 := s - offset2; len(blk.sequences) > 2 && load3232(src, o2) == uint32(cv) {
			// We have at least 4 byte match.
			// No need to check backwards. We come straight from a match
			//l := 4 + e.matchlenNoHist(s+4, o2+4, src)
			l := 4 + int32(matchLen(src[s+4:], src[o2+4:]))

			// Store this, since we have it.
			nextHash := hash6(cv, hashLog)
			e.table[nextHash] = tableEntry{offset: s + e.cur, val: uint32(cv)}
			seq.matchLen = uint32(l) - zstdMinMatch
			seq.litLen = 0
			// Since litlen is always 0, this is offset 1.
			seq.offset = 1
			s += l
			nextEmit = s
			if debugSequences {
				println("sequence", seq, "next s:", s)
			}
			blk.sequences = append(blk.sequences, seq)

			// Swap offset 1 and 2.
			offset1, offset2 = offset2, offset1
			if s >= sLimit {
				break encodeLoop
			}
			// Prepare next loop.
			cv = load6432(src, s)
		}
	}

	if int(nextEmit) < len(src) {
		blk.literals = append(blk.literals, src[nextEmit:]...)
		blk.extraLits = len(src) - int(nextEmit)
	}
	if debug {
		println("returning, recent offsets:", blk.recentOffsets, "extra literals:", blk.extraLits)
	}
}

func (e *fastEncoder) addBlock(src []byte) int32 {
	if debugAsserts && e.cur > bufferReset {
		panic(fmt.Sprintf("ecur (%d) > buffer reset (%d)", e.cur, bufferReset))
	}
	// check if we have space already
	if len(e.hist)+len(src) > cap(e.hist) {
		if cap(e.hist) == 0 {
			l := e.maxMatchOff * 2
			// Make it at least 1MB.
			if l < 1<<20 {
				l = 1 << 20
			}
			e.hist = make([]byte, 0, l)
		} else {
			if cap(e.hist) < int(e.maxMatchOff*2) {
				panic("unexpected buffer size")
			}
			// Move down
			offset := int32(len(e.hist)) - e.maxMatchOff
			copy(e.hist[0:e.maxMatchOff], e.hist[offset:])
			e.cur += offset
			e.hist = e.hist[:e.maxMatchOff]
		}
	}
	s := int32(len(e.hist))
	e.hist = append(e.hist, src...)
	return s
}

// useBlock will replace the block with the provided one,
// but transfer recent offsets from the previous.
func (e *fastEncoder) UseBlock(enc *blockEnc) {
	enc.reset(e.blk)
	e.blk = enc
}

func (e *fastEncoder) matchlenNoHist(s, t int32, src []byte) int32 {
	// Extend the match to be as long as possible.
	return int32(matchLen(src[s:], src[t:]))
}

func (e *fastEncoder) matchlen(s, t int32, src []byte) int32 {
	if debugAsserts {
		if s < 0 {
			err := fmt.Sprintf("s (%d) < 0", s)
			panic(err)
		}
		if t < 0 {
			err := fmt.Sprintf("s (%d) < 0", s)
			panic(err)
		}
		if s-t > e.maxMatchOff {
			err := fmt.Sprintf("s (%d) - t (%d) > maxMatchOff (%d)", s, t, e.maxMatchOff)
			panic(err)
		}
	}
	s1 := int(s) + maxMatchLength - 4
	if s1 > len(src) {
		s1 = len(src)
	}

	// Extend the match to be as long as possible.
	return int32(matchLen(src[s:s1], src[t:]))
}

// Reset the encoding table.
func (e *fastEncoder) Reset() {
	if e.blk == nil {
		e.blk = &blockEnc{}
		e.blk.init()
	} else {
		e.blk.reset(nil)
	}
	e.blk.initNewEncode()
	if e.crc == nil {
		e.crc = xxhash.New()
	} else {
		e.crc.Reset()
	}
	if cap(e.hist) < int(e.maxMatchOff*2) {
		l := e.maxMatchOff * 2
		// Make it at least 1MB.
		if l < 1<<20 {
			l = 1 << 20
		}
		e.hist = make([]byte, 0, l)
	}
	// We offset current position so everything will be out of reach.
	// If above reset line, history will be purged.
	if e.cur < bufferReset {
		e.cur += e.maxMatchOff + int32(len(e.hist))
	}
	e.hist = e.hist[:0]
}
