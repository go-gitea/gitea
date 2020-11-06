/*
 * Branch/Call/Jump (BCJ) filter decoders
 *
 * Authors: Lasse Collin <lasse.collin@tukaani.org>
 *          Igor Pavlov <http://7-zip.org/>
 *
 * Translation to Go: Michael Cross <https://github.com/xi2>
 *
 * This file has been put into the public domain.
 * You can do whatever you want with this file.
 */

package xz

/* from linux/lib/xz/xz_dec_bcj.c *************************************/

type xzDecBCJ struct {
	/* Type of the BCJ filter being used */
	typ xzFilterID
	/*
	 * Return value of the next filter in the chain. We need to preserve
	 * this information across calls, because we must not call the next
	 * filter anymore once it has returned xzStreamEnd
	 */
	ret xzRet
	/*
	 * Absolute position relative to the beginning of the uncompressed
	 * data (in a single .xz Block).
	 */
	pos int
	/* x86 filter state */
	x86PrevMask uint32
	/* Temporary space to hold the variables from xzBuf */
	out    []byte
	outPos int
	temp   struct {
		/* Amount of already filtered data in the beginning of buf */
		filtered int
		/*
		 * Buffer to hold a mix of filtered and unfiltered data. This
		 * needs to be big enough to hold Alignment + 2 * Look-ahead:
		 *
		 * Type         Alignment   Look-ahead
		 * x86              1           4
		 * PowerPC          4           0
		 * IA-64           16           0
		 * ARM              4           0
		 * ARM-Thumb        2           2
		 * SPARC            4           0
		 */
		buf      []byte // slice buf will be backed by bufArray
		bufArray [16]byte
	}
}

/*
 * This is used to test the most significant byte of a memory address
 * in an x86 instruction.
 */
func bcjX86TestMSByte(b byte) bool {
	return b == 0x00 || b == 0xff
}

func bcjX86Filter(s *xzDecBCJ, buf []byte) int {
	var maskToAllowedStatus = []bool{
		true, true, true, false, true, false, false, false,
	}
	var maskToBitNum = []byte{0, 1, 2, 2, 3, 3, 3, 3}
	var i int
	var prevPos int = -1
	var prevMask uint32 = s.x86PrevMask
	var src uint32
	var dest uint32
	var j uint32
	var b byte
	if len(buf) <= 4 {
		return 0
	}
	for i = 0; i < len(buf)-4; i++ {
		if buf[i]&0xfe != 0xe8 {
			continue
		}
		prevPos = i - prevPos
		if prevPos > 3 {
			prevMask = 0
		} else {
			prevMask = (prevMask << (uint(prevPos) - 1)) & 7
			if prevMask != 0 {
				b = buf[i+4-int(maskToBitNum[prevMask])]
				if !maskToAllowedStatus[prevMask] || bcjX86TestMSByte(b) {
					prevPos = i
					prevMask = prevMask<<1 | 1
					continue
				}
			}
		}
		prevPos = i
		if bcjX86TestMSByte(buf[i+4]) {
			src = getLE32(buf[i+1:])
			for {
				dest = src - uint32(s.pos+i+5)
				if prevMask == 0 {
					break
				}
				j = uint32(maskToBitNum[prevMask]) * 8
				b = byte(dest >> (24 - j))
				if !bcjX86TestMSByte(b) {
					break
				}
				src = dest ^ (1<<(32-j) - 1)
			}
			dest &= 0x01FFFFFF
			dest |= 0 - dest&0x01000000
			putLE32(dest, buf[i+1:])
			i += 4
		} else {
			prevMask = prevMask<<1 | 1
		}
	}
	prevPos = i - prevPos
	if prevPos > 3 {
		s.x86PrevMask = 0
	} else {
		s.x86PrevMask = prevMask << (uint(prevPos) - 1)
	}
	return i
}

func bcjPowerPCFilter(s *xzDecBCJ, buf []byte) int {
	var i int
	var instr uint32
	for i = 0; i+4 <= len(buf); i += 4 {
		instr = getBE32(buf[i:])
		if instr&0xFC000003 == 0x48000001 {
			instr &= 0x03FFFFFC
			instr -= uint32(s.pos + i)
			instr &= 0x03FFFFFC
			instr |= 0x48000001
			putBE32(instr, buf[i:])
		}
	}
	return i
}

var bcjIA64BranchTable = [...]byte{
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	4, 4, 6, 6, 0, 0, 7, 7,
	4, 4, 0, 0, 4, 4, 0, 0,
}

func bcjIA64Filter(s *xzDecBCJ, buf []byte) int {
	var branchTable = bcjIA64BranchTable[:]
	/*
	 * The local variables take a little bit stack space, but it's less
	 * than what LZMA2 decoder takes, so it doesn't make sense to reduce
	 * stack usage here without doing that for the LZMA2 decoder too.
	 */
	/* Loop counters */
	var i int
	var j int
	/* Instruction slot (0, 1, or 2) in the 128-bit instruction word */
	var slot uint32
	/* Bitwise offset of the instruction indicated by slot */
	var bitPos uint32
	/* bit_pos split into byte and bit parts */
	var bytePos uint32
	var bitRes uint32
	/* Address part of an instruction */
	var addr uint32
	/* Mask used to detect which instructions to convert */
	var mask uint32
	/* 41-bit instruction stored somewhere in the lowest 48 bits */
	var instr uint64
	/* Instruction normalized with bit_res for easier manipulation */
	var norm uint64
	for i = 0; i+16 <= len(buf); i += 16 {
		mask = uint32(branchTable[buf[i]&0x1f])
		for slot, bitPos = 0, 5; slot < 3; slot, bitPos = slot+1, bitPos+41 {
			if (mask>>slot)&1 == 0 {
				continue
			}
			bytePos = bitPos >> 3
			bitRes = bitPos & 7
			instr = 0
			for j = 0; j < 6; j++ {
				instr |= uint64(buf[i+j+int(bytePos)]) << (8 * uint(j))
			}
			norm = instr >> bitRes
			if (norm>>37)&0x0f == 0x05 && (norm>>9)&0x07 == 0 {
				addr = uint32((norm >> 13) & 0x0fffff)
				addr |= (uint32(norm>>36) & 1) << 20
				addr <<= 4
				addr -= uint32(s.pos + i)
				addr >>= 4
				norm &= ^(uint64(0x8fffff) << 13)
				norm |= uint64(addr&0x0fffff) << 13
				norm |= uint64(addr&0x100000) << (36 - 20)
				instr &= 1<<bitRes - 1
				instr |= norm << bitRes
				for j = 0; j < 6; j++ {
					buf[i+j+int(bytePos)] = byte(instr >> (8 * uint(j)))
				}
			}
		}
	}
	return i
}

func bcjARMFilter(s *xzDecBCJ, buf []byte) int {
	var i int
	var addr uint32
	for i = 0; i+4 <= len(buf); i += 4 {
		if buf[i+3] == 0xeb {
			addr = uint32(buf[i]) | uint32(buf[i+1])<<8 |
				uint32(buf[i+2])<<16
			addr <<= 2
			addr -= uint32(s.pos + i + 8)
			addr >>= 2
			buf[i] = byte(addr)
			buf[i+1] = byte(addr >> 8)
			buf[i+2] = byte(addr >> 16)
		}
	}
	return i
}

func bcjARMThumbFilter(s *xzDecBCJ, buf []byte) int {
	var i int
	var addr uint32
	for i = 0; i+4 <= len(buf); i += 2 {
		if buf[i+1]&0xf8 == 0xf0 && buf[i+3]&0xf8 == 0xf8 {
			addr = uint32(buf[i+1]&0x07)<<19 |
				uint32(buf[i])<<11 |
				uint32(buf[i+3]&0x07)<<8 |
				uint32(buf[i+2])
			addr <<= 1
			addr -= uint32(s.pos + i + 4)
			addr >>= 1
			buf[i+1] = byte(0xf0 | (addr>>19)&0x07)
			buf[i] = byte(addr >> 11)
			buf[i+3] = byte(0xf8 | (addr>>8)&0x07)
			buf[i+2] = byte(addr)
			i += 2
		}
	}
	return i
}

func bcjSPARCFilter(s *xzDecBCJ, buf []byte) int {
	var i int
	var instr uint32
	for i = 0; i+4 <= len(buf); i += 4 {
		instr = getBE32(buf[i:])
		if instr>>22 == 0x100 || instr>>22 == 0x1ff {
			instr <<= 2
			instr -= uint32(s.pos + i)
			instr >>= 2
			instr = (0x40000000 - instr&0x400000) |
				0x40000000 | (instr & 0x3FFFFF)
			putBE32(instr, buf[i:])
		}
	}
	return i
}

/*
 * Apply the selected BCJ filter. Update *pos and s.pos to match the amount
 * of data that got filtered.
 */
func bcjApply(s *xzDecBCJ, buf []byte, pos *int) {
	var filtered int
	buf = buf[*pos:]
	switch s.typ {
	case idBCJX86:
		filtered = bcjX86Filter(s, buf)
	case idBCJPowerPC:
		filtered = bcjPowerPCFilter(s, buf)
	case idBCJIA64:
		filtered = bcjIA64Filter(s, buf)
	case idBCJARM:
		filtered = bcjARMFilter(s, buf)
	case idBCJARMThumb:
		filtered = bcjARMThumbFilter(s, buf)
	case idBCJSPARC:
		filtered = bcjSPARCFilter(s, buf)
	default:
		/* Never reached */
	}
	*pos += filtered
	s.pos += filtered
}

/*
 * Flush pending filtered data from temp to the output buffer.
 * Move the remaining mixture of possibly filtered and unfiltered
 * data to the beginning of temp.
 */
func bcjFlush(s *xzDecBCJ, b *xzBuf) {
	var copySize int
	copySize = len(b.out) - b.outPos
	if copySize > s.temp.filtered {
		copySize = s.temp.filtered
	}
	copy(b.out[b.outPos:], s.temp.buf[:copySize])
	b.outPos += copySize
	s.temp.filtered -= copySize
	copy(s.temp.buf, s.temp.buf[copySize:])
	s.temp.buf = s.temp.buf[:len(s.temp.buf)-copySize]
}

/*
 * Decode raw stream which has a BCJ filter as the first filter.
 *
 * The BCJ filter functions are primitive in sense that they process the
 * data in chunks of 1-16 bytes. To hide this issue, this function does
 * some buffering.
 */
func xzDecBCJRun(s *xzDecBCJ, b *xzBuf, chain func(*xzBuf) xzRet) xzRet {
	var outStart int
	/*
	 * Flush pending already filtered data to the output buffer. Return
	 * immediately if we couldn't flush everything, or if the next
	 * filter in the chain had already returned xzStreamEnd.
	 */
	if s.temp.filtered > 0 {
		bcjFlush(s, b)
		if s.temp.filtered > 0 {
			return xzOK
		}
		if s.ret == xzStreamEnd {
			return xzStreamEnd
		}
	}
	/*
	 * If we have more output space than what is currently pending in
	 * temp, copy the unfiltered data from temp to the output buffer
	 * and try to fill the output buffer by decoding more data from the
	 * next filter in the chain. Apply the BCJ filter on the new data
	 * in the output buffer. If everything cannot be filtered, copy it
	 * to temp and rewind the output buffer position accordingly.
	 *
	 * This needs to be always run when len(temp.buf) == 0 to handle a special
	 * case where the output buffer is full and the next filter has no
	 * more output coming but hasn't returned xzStreamEnd yet.
	 */
	if len(s.temp.buf) < len(b.out)-b.outPos || len(s.temp.buf) == 0 {
		outStart = b.outPos
		copy(b.out[b.outPos:], s.temp.buf)
		b.outPos += len(s.temp.buf)
		s.ret = chain(b)
		if s.ret != xzStreamEnd && s.ret != xzOK {
			return s.ret
		}
		bcjApply(s, b.out[:b.outPos], &outStart)
		/*
		 * As an exception, if the next filter returned xzStreamEnd,
		 * we can do that too, since the last few bytes that remain
		 * unfiltered are meant to remain unfiltered.
		 */
		if s.ret == xzStreamEnd {
			return xzStreamEnd
		}
		s.temp.buf = s.temp.bufArray[:b.outPos-outStart]
		b.outPos -= len(s.temp.buf)
		copy(s.temp.buf, b.out[b.outPos:])
		/*
		 * If there wasn't enough input to the next filter to fill
		 * the output buffer with unfiltered data, there's no point
		 * to try decoding more data to temp.
		 */
		if b.outPos+len(s.temp.buf) < len(b.out) {
			return xzOK
		}
	}
	/*
	 * We have unfiltered data in temp. If the output buffer isn't full
	 * yet, try to fill the temp buffer by decoding more data from the
	 * next filter. Apply the BCJ filter on temp. Then we hopefully can
	 * fill the actual output buffer by copying filtered data from temp.
	 * A mix of filtered and unfiltered data may be left in temp; it will
	 * be taken care on the next call to this function.
	 */
	if b.outPos < len(b.out) {
		/* Make b.out temporarily point to s.temp. */
		s.out = b.out
		s.outPos = b.outPos
		b.out = s.temp.bufArray[:]
		b.outPos = len(s.temp.buf)
		s.ret = chain(b)
		s.temp.buf = s.temp.bufArray[:b.outPos]
		b.out = s.out
		b.outPos = s.outPos
		if s.ret != xzOK && s.ret != xzStreamEnd {
			return s.ret
		}
		bcjApply(s, s.temp.buf, &s.temp.filtered)
		/*
		 * If the next filter returned xzStreamEnd, we mark that
		 * everything is filtered, since the last unfiltered bytes
		 * of the stream are meant to be left as is.
		 */
		if s.ret == xzStreamEnd {
			s.temp.filtered = len(s.temp.buf)
		}
		bcjFlush(s, b)
		if s.temp.filtered > 0 {
			return xzOK
		}
	}
	return s.ret
}

/*
 * Allocate memory for BCJ decoders. xzDecBCJReset must be used before
 * calling xzDecBCJRun.
 */
func xzDecBCJCreate() *xzDecBCJ {
	return new(xzDecBCJ)
}

/*
 * Decode the Filter ID of a BCJ filter and check the start offset is
 * valid. Returns xzOK if the given Filter ID and offset is
 * supported. Otherwise xzOptionsError is returned.
 */
func xzDecBCJReset(s *xzDecBCJ, id xzFilterID, offset int) xzRet {
	switch id {
	case idBCJX86:
	case idBCJPowerPC:
	case idBCJIA64:
	case idBCJARM:
	case idBCJARMThumb:
	case idBCJSPARC:
	default:
		/* Unsupported Filter ID */
		return xzOptionsError
	}
	// check offset is a multiple of alignment
	switch id {
	case idBCJPowerPC, idBCJARM, idBCJSPARC:
		if offset%4 != 0 {
			return xzOptionsError
		}
	case idBCJIA64:
		if offset%16 != 0 {
			return xzOptionsError
		}
	case idBCJARMThumb:
		if offset%2 != 0 {
			return xzOptionsError
		}
	}
	s.typ = id
	s.ret = xzOK
	s.pos = offset
	s.x86PrevMask = 0
	s.temp.filtered = 0
	s.temp.buf = nil
	return xzOK
}
