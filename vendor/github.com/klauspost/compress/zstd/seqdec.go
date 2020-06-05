// Copyright 2019+ Klaus Post. All rights reserved.
// License information can be found in the LICENSE file.
// Based on work by Yann Collet, released under BSD License.

package zstd

import (
	"errors"
	"fmt"
	"io"
)

type seq struct {
	litLen   uint32
	matchLen uint32
	offset   uint32

	// Codes are stored here for the encoder
	// so they only have to be looked up once.
	llCode, mlCode, ofCode uint8
}

func (s seq) String() string {
	if s.offset <= 3 {
		if s.offset == 0 {
			return fmt.Sprint("litLen:", s.litLen, ", matchLen:", s.matchLen+zstdMinMatch, ", offset: INVALID (0)")
		}
		return fmt.Sprint("litLen:", s.litLen, ", matchLen:", s.matchLen+zstdMinMatch, ", offset:", s.offset, " (repeat)")
	}
	return fmt.Sprint("litLen:", s.litLen, ", matchLen:", s.matchLen+zstdMinMatch, ", offset:", s.offset-3, " (new)")
}

type seqCompMode uint8

const (
	compModePredefined seqCompMode = iota
	compModeRLE
	compModeFSE
	compModeRepeat
)

type sequenceDec struct {
	// decoder keeps track of the current state and updates it from the bitstream.
	fse    *fseDecoder
	state  fseState
	repeat bool
}

// init the state of the decoder with input from stream.
func (s *sequenceDec) init(br *bitReader) error {
	if s.fse == nil {
		return errors.New("sequence decoder not defined")
	}
	s.state.init(br, s.fse.actualTableLog, s.fse.dt[:1<<s.fse.actualTableLog])
	return nil
}

// sequenceDecs contains all 3 sequence decoders and their state.
type sequenceDecs struct {
	litLengths   sequenceDec
	offsets      sequenceDec
	matchLengths sequenceDec
	prevOffset   [3]int
	hist         []byte
	literals     []byte
	out          []byte
	maxBits      uint8
}

// initialize all 3 decoders from the stream input.
func (s *sequenceDecs) initialize(br *bitReader, hist *history, literals, out []byte) error {
	if err := s.litLengths.init(br); err != nil {
		return errors.New("litLengths:" + err.Error())
	}
	if err := s.offsets.init(br); err != nil {
		return errors.New("offsets:" + err.Error())
	}
	if err := s.matchLengths.init(br); err != nil {
		return errors.New("matchLengths:" + err.Error())
	}
	s.literals = literals
	s.hist = hist.b
	s.prevOffset = hist.recentOffsets
	s.maxBits = s.litLengths.fse.maxBits + s.offsets.fse.maxBits + s.matchLengths.fse.maxBits
	s.out = out
	return nil
}

// decode sequences from the stream with the provided history.
func (s *sequenceDecs) decode(seqs int, br *bitReader, hist []byte) error {
	startSize := len(s.out)
	// Grab full sizes tables, to avoid bounds checks.
	llTable, mlTable, ofTable := s.litLengths.fse.dt[:maxTablesize], s.matchLengths.fse.dt[:maxTablesize], s.offsets.fse.dt[:maxTablesize]
	llState, mlState, ofState := s.litLengths.state.state, s.matchLengths.state.state, s.offsets.state.state

	for i := seqs - 1; i >= 0; i-- {
		if br.overread() {
			printf("reading sequence %d, exceeded available data\n", seqs-i)
			return io.ErrUnexpectedEOF
		}
		var litLen, matchOff, matchLen int
		if br.off > 4+((maxOffsetBits+16+16)>>3) {
			litLen, matchOff, matchLen = s.nextFast(br, llState, mlState, ofState)
			br.fillFast()
		} else {
			litLen, matchOff, matchLen = s.next(br, llState, mlState, ofState)
			br.fill()
		}

		if debugSequences {
			println("Seq", seqs-i-1, "Litlen:", litLen, "matchOff:", matchOff, "(abs) matchLen:", matchLen)
		}

		if litLen > len(s.literals) {
			return fmt.Errorf("unexpected literal count, want %d bytes, but only %d is available", litLen, len(s.literals))
		}
		size := litLen + matchLen + len(s.out)
		if size-startSize > maxBlockSize {
			return fmt.Errorf("output (%d) bigger than max block size", size)
		}
		if size > cap(s.out) {
			// Not enough size, will be extremely rarely triggered,
			// but could be if destination slice is too small for sync operations.
			// We add maxBlockSize to the capacity.
			s.out = append(s.out, make([]byte, maxBlockSize)...)
			s.out = s.out[:len(s.out)-maxBlockSize]
		}
		if matchLen > maxMatchLen {
			return fmt.Errorf("match len (%d) bigger than max allowed length", matchLen)
		}
		if matchOff > len(s.out)+len(hist)+litLen {
			return fmt.Errorf("match offset (%d) bigger than current history (%d)", matchOff, len(s.out)+len(hist)+litLen)
		}
		if matchOff == 0 && matchLen > 0 {
			return fmt.Errorf("zero matchoff and matchlen > 0")
		}

		s.out = append(s.out, s.literals[:litLen]...)
		s.literals = s.literals[litLen:]
		out := s.out

		// Copy from history.
		// TODO: Blocks without history could be made to ignore this completely.
		if v := matchOff - len(s.out); v > 0 {
			// v is the start position in history from end.
			start := len(s.hist) - v
			if matchLen > v {
				// Some goes into current block.
				// Copy remainder of history
				out = append(out, s.hist[start:]...)
				matchOff -= v
				matchLen -= v
			} else {
				out = append(out, s.hist[start:start+matchLen]...)
				matchLen = 0
			}
		}
		// We must be in current buffer now
		if matchLen > 0 {
			start := len(s.out) - matchOff
			if matchLen <= len(s.out)-start {
				// No overlap
				out = append(out, s.out[start:start+matchLen]...)
			} else {
				// Overlapping copy
				// Extend destination slice and copy one byte at the time.
				out = out[:len(out)+matchLen]
				src := out[start : start+matchLen]
				// Destination is the space we just added.
				dst := out[len(out)-matchLen:]
				dst = dst[:len(src)]
				for i := range src {
					dst[i] = src[i]
				}
			}
		}
		s.out = out
		if i == 0 {
			// This is the last sequence, so we shouldn't update state.
			break
		}

		// Manually inlined, ~ 5-20% faster
		// Update all 3 states at once. Approx 20% faster.
		nBits := llState.nbBits() + mlState.nbBits() + ofState.nbBits()
		if nBits == 0 {
			llState = llTable[llState.newState()&maxTableMask]
			mlState = mlTable[mlState.newState()&maxTableMask]
			ofState = ofTable[ofState.newState()&maxTableMask]
		} else {
			bits := br.getBitsFast(nBits)
			lowBits := uint16(bits >> ((ofState.nbBits() + mlState.nbBits()) & 31))
			llState = llTable[(llState.newState()+lowBits)&maxTableMask]

			lowBits = uint16(bits >> (ofState.nbBits() & 31))
			lowBits &= bitMask[mlState.nbBits()&15]
			mlState = mlTable[(mlState.newState()+lowBits)&maxTableMask]

			lowBits = uint16(bits) & bitMask[ofState.nbBits()&15]
			ofState = ofTable[(ofState.newState()+lowBits)&maxTableMask]
		}
	}

	// Add final literals
	s.out = append(s.out, s.literals...)
	return nil
}

// update states, at least 27 bits must be available.
func (s *sequenceDecs) update(br *bitReader) {
	// Max 8 bits
	s.litLengths.state.next(br)
	// Max 9 bits
	s.matchLengths.state.next(br)
	// Max 8 bits
	s.offsets.state.next(br)
}

var bitMask [16]uint16

func init() {
	for i := range bitMask[:] {
		bitMask[i] = uint16((1 << uint(i)) - 1)
	}
}

// update states, at least 27 bits must be available.
func (s *sequenceDecs) updateAlt(br *bitReader) {
	// Update all 3 states at once. Approx 20% faster.
	a, b, c := s.litLengths.state.state, s.matchLengths.state.state, s.offsets.state.state

	nBits := a.nbBits() + b.nbBits() + c.nbBits()
	if nBits == 0 {
		s.litLengths.state.state = s.litLengths.state.dt[a.newState()]
		s.matchLengths.state.state = s.matchLengths.state.dt[b.newState()]
		s.offsets.state.state = s.offsets.state.dt[c.newState()]
		return
	}
	bits := br.getBitsFast(nBits)
	lowBits := uint16(bits >> ((c.nbBits() + b.nbBits()) & 31))
	s.litLengths.state.state = s.litLengths.state.dt[a.newState()+lowBits]

	lowBits = uint16(bits >> (c.nbBits() & 31))
	lowBits &= bitMask[b.nbBits()&15]
	s.matchLengths.state.state = s.matchLengths.state.dt[b.newState()+lowBits]

	lowBits = uint16(bits) & bitMask[c.nbBits()&15]
	s.offsets.state.state = s.offsets.state.dt[c.newState()+lowBits]
}

// nextFast will return new states when there are at least 4 unused bytes left on the stream when done.
func (s *sequenceDecs) nextFast(br *bitReader, llState, mlState, ofState decSymbol) (ll, mo, ml int) {
	// Final will not read from stream.
	ll, llB := llState.final()
	ml, mlB := mlState.final()
	mo, moB := ofState.final()

	// extra bits are stored in reverse order.
	br.fillFast()
	mo += br.getBits(moB)
	if s.maxBits > 32 {
		br.fillFast()
	}
	ml += br.getBits(mlB)
	ll += br.getBits(llB)

	if moB > 1 {
		s.prevOffset[2] = s.prevOffset[1]
		s.prevOffset[1] = s.prevOffset[0]
		s.prevOffset[0] = mo
		return
	}
	// mo = s.adjustOffset(mo, ll, moB)
	// Inlined for rather big speedup
	if ll == 0 {
		// There is an exception though, when current sequence's literals_length = 0.
		// In this case, repeated offsets are shifted by one, so an offset_value of 1 means Repeated_Offset2,
		// an offset_value of 2 means Repeated_Offset3, and an offset_value of 3 means Repeated_Offset1 - 1_byte.
		mo++
	}

	if mo == 0 {
		mo = s.prevOffset[0]
		return
	}
	var temp int
	if mo == 3 {
		temp = s.prevOffset[0] - 1
	} else {
		temp = s.prevOffset[mo]
	}

	if temp == 0 {
		// 0 is not valid; input is corrupted; force offset to 1
		println("temp was 0")
		temp = 1
	}

	if mo != 1 {
		s.prevOffset[2] = s.prevOffset[1]
	}
	s.prevOffset[1] = s.prevOffset[0]
	s.prevOffset[0] = temp
	mo = temp
	return
}

func (s *sequenceDecs) next(br *bitReader, llState, mlState, ofState decSymbol) (ll, mo, ml int) {
	// Final will not read from stream.
	ll, llB := llState.final()
	ml, mlB := mlState.final()
	mo, moB := ofState.final()

	// extra bits are stored in reverse order.
	br.fill()
	if s.maxBits <= 32 {
		mo += br.getBits(moB)
		ml += br.getBits(mlB)
		ll += br.getBits(llB)
	} else {
		mo += br.getBits(moB)
		br.fill()
		// matchlength+literal length, max 32 bits
		ml += br.getBits(mlB)
		ll += br.getBits(llB)

	}
	mo = s.adjustOffset(mo, ll, moB)
	return
}

func (s *sequenceDecs) adjustOffset(offset, litLen int, offsetB uint8) int {
	if offsetB > 1 {
		s.prevOffset[2] = s.prevOffset[1]
		s.prevOffset[1] = s.prevOffset[0]
		s.prevOffset[0] = offset
		return offset
	}

	if litLen == 0 {
		// There is an exception though, when current sequence's literals_length = 0.
		// In this case, repeated offsets are shifted by one, so an offset_value of 1 means Repeated_Offset2,
		// an offset_value of 2 means Repeated_Offset3, and an offset_value of 3 means Repeated_Offset1 - 1_byte.
		offset++
	}

	if offset == 0 {
		return s.prevOffset[0]
	}
	var temp int
	if offset == 3 {
		temp = s.prevOffset[0] - 1
	} else {
		temp = s.prevOffset[offset]
	}

	if temp == 0 {
		// 0 is not valid; input is corrupted; force offset to 1
		println("temp was 0")
		temp = 1
	}

	if offset != 1 {
		s.prevOffset[2] = s.prevOffset[1]
	}
	s.prevOffset[1] = s.prevOffset[0]
	s.prevOffset[0] = temp
	return temp
}

// mergeHistory will merge history.
func (s *sequenceDecs) mergeHistory(hist *sequenceDecs) (*sequenceDecs, error) {
	for i := uint(0); i < 3; i++ {
		var sNew, sHist *sequenceDec
		switch i {
		default:
			// same as "case 0":
			sNew = &s.litLengths
			sHist = &hist.litLengths
		case 1:
			sNew = &s.offsets
			sHist = &hist.offsets
		case 2:
			sNew = &s.matchLengths
			sHist = &hist.matchLengths
		}
		if sNew.repeat {
			if sHist.fse == nil {
				return nil, fmt.Errorf("sequence stream %d, repeat requested, but no history", i)
			}
			continue
		}
		if sNew.fse == nil {
			return nil, fmt.Errorf("sequence stream %d, no fse found", i)
		}
		if sHist.fse != nil && !sHist.fse.preDefined {
			fseDecoderPool.Put(sHist.fse)
		}
		sHist.fse = sNew.fse
	}
	return hist, nil
}
