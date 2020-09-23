/*
 * LZMA2 decoder
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

/* from linux/lib/xz/xz_lzma2.h ***************************************/

/* Range coder constants */
const (
	rcShiftBits         = 8
	rcTopBits           = 24
	rcTopValue          = 1 << rcTopBits
	rcBitModelTotalBits = 11
	rcBitModelTotal     = 1 << rcBitModelTotalBits
	rcMoveBits          = 5
)

/*
 * Maximum number of position states. A position state is the lowest pb
 * number of bits of the current uncompressed offset. In some places there
 * are different sets of probabilities for different position states.
 */
const posStatesMax = 1 << 4

/*
 * lzmaState is used to track which LZMA symbols have occurred most recently
 * and in which order. This information is used to predict the next symbol.
 *
 * Symbols:
 *  - Literal: One 8-bit byte
 *  - Match: Repeat a chunk of data at some distance
 *  - Long repeat: Multi-byte match at a recently seen distance
 *  - Short repeat: One-byte repeat at a recently seen distance
 *
 * The symbol names are in from STATE-oldest-older-previous. REP means
 * either short or long repeated match, and NONLIT means any non-literal.
 */
type lzmaState int

const (
	stateLitLit lzmaState = iota
	stateMatchLitLit
	stateRepLitLit
	stateShortrepLitLit
	stateMatchLit
	stateRepList
	stateShortrepLit
	stateLitMatch
	stateLitLongrep
	stateLitShortrep
	stateNonlitMatch
	stateNonlitRep
)

/* Total number of states */
const states = 12

/* The lowest 7 states indicate that the previous state was a literal. */
const litStates = 7

/* Indicate that the latest symbol was a literal. */
func lzmaStateLiteral(state *lzmaState) {
	switch {
	case *state <= stateShortrepLitLit:
		*state = stateLitLit
	case *state <= stateLitShortrep:
		*state -= 3
	default:
		*state -= 6
	}
}

/* Indicate that the latest symbol was a match. */
func lzmaStateMatch(state *lzmaState) {
	if *state < litStates {
		*state = stateLitMatch
	} else {
		*state = stateNonlitMatch
	}
}

/* Indicate that the latest state was a long repeated match. */
func lzmaStateLongRep(state *lzmaState) {
	if *state < litStates {
		*state = stateLitLongrep
	} else {
		*state = stateNonlitRep
	}
}

/* Indicate that the latest symbol was a short match. */
func lzmaStateShortRep(state *lzmaState) {
	if *state < litStates {
		*state = stateLitShortrep
	} else {
		*state = stateNonlitRep
	}
}

/* Test if the previous symbol was a literal. */
func lzmaStateIsLiteral(state lzmaState) bool {
	return state < litStates
}

/* Each literal coder is divided in three sections:
 *   - 0x001-0x0FF: Without match byte
 *   - 0x101-0x1FF: With match byte; match bit is 0
 *   - 0x201-0x2FF: With match byte; match bit is 1
 *
 * Match byte is used when the previous LZMA symbol was something else than
 * a literal (that is, it was some kind of match).
 */
const literalCoderSize = 0x300

/* Maximum number of literal coders */
const literalCodersMax = 1 << 4

/* Minimum length of a match is two bytes. */
const matchLenMin = 2

/* Match length is encoded with 4, 5, or 10 bits.
 *
 * Length   Bits
 *  2-9      4 = Choice=0 + 3 bits
 * 10-17     5 = Choice=1 + Choice2=0 + 3 bits
 * 18-273   10 = Choice=1 + Choice2=1 + 8 bits
 */
const (
	lenLowBits     = 3
	lenLowSymbols  = 1 << lenLowBits
	lenMidBits     = 3
	lenMidSymbols  = 1 << lenMidBits
	lenHighBits    = 8
	lenHighSymbols = 1 << lenHighBits
)

/*
 * Different sets of probabilities are used for match distances that have
 * very short match length: Lengths of 2, 3, and 4 bytes have a separate
 * set of probabilities for each length. The matches with longer length
 * use a shared set of probabilities.
 */
const distStates = 4

/*
 * Get the index of the appropriate probability array for decoding
 * the distance slot.
 */
func lzmaGetDistState(len uint32) uint32 {
	if len < distStates+matchLenMin {
		return len - matchLenMin
	} else {
		return distStates - 1
	}
}

/*
 * The highest two bits of a 32-bit match distance are encoded using six bits.
 * This six-bit value is called a distance slot. This way encoding a 32-bit
 * value takes 6-36 bits, larger values taking more bits.
 */
const (
	distSlotBits = 6
	distSlots    = 1 << distSlotBits
)

/* Match distances up to 127 are fully encoded using probabilities. Since
 * the highest two bits (distance slot) are always encoded using six bits,
 * the distances 0-3 don't need any additional bits to encode, since the
 * distance slot itself is the same as the actual distance. distModelStart
 * indicates the first distance slot where at least one additional bit is
 * needed.
 */
const distModelStart = 4

/*
 * Match distances greater than 127 are encoded in three pieces:
 *   - distance slot: the highest two bits
 *   - direct bits: 2-26 bits below the highest two bits
 *   - alignment bits: four lowest bits
 *
 * Direct bits don't use any probabilities.
 *
 * The distance slot value of 14 is for distances 128-191.
 */
const distModelEnd = 14

/* Distance slots that indicate a distance <= 127. */
const (
	fullDistancesBits = distModelEnd / 2
	fullDistances     = 1 << fullDistancesBits
)

/*
 * For match distances greater than 127, only the highest two bits and the
 * lowest four bits (alignment) is encoded using probabilities.
 */
const (
	alignBits = 4
	alignSize = 1 << alignBits
)

/* from linux/lib/xz/xz_dec_lzma2.c ***********************************/

/*
 * Range decoder initialization eats the first five bytes of each LZMA chunk.
 */
const rcInitBytes = 5

/*
 * Minimum number of usable input buffer to safely decode one LZMA symbol.
 * The worst case is that we decode 22 bits using probabilities and 26
 * direct bits. This may decode at maximum of 20 bytes of input. However,
 * lzmaMain does an extra normalization before returning, thus we
 * need to put 21 here.
 */
const lzmaInRequired = 21

/*
 * Dictionary (history buffer)
 *
 * These are always true:
 *    start <= pos <= full <= end
 *    pos <= limit <= end
 *    end == size
 *    size <= sizeMax
 *    len(buf) <= size
 */
type dictionary struct {
	/* The history buffer */
	buf []byte
	/* Old position in buf (before decoding more data) */
	start uint32
	/* Position in buf */
	pos uint32
	/*
	 * How full dictionary is. This is used to detect corrupt input that
	 * would read beyond the beginning of the uncompressed stream.
	 */
	full uint32
	/* Write limit; we don't write to buf[limit] or later bytes. */
	limit uint32
	/*
	 * End of the dictionary buffer. This is the same as the
	 * dictionary size.
	 */
	end uint32
	/*
	 * Size of the dictionary as specified in Block Header. This is used
	 * together with "full" to detect corrupt input that would make us
	 * read beyond the beginning of the uncompressed stream.
	 */
	size uint32
	/* Maximum allowed dictionary size. */
	sizeMax uint32
}

/* Range decoder */
type rcDec struct {
	rnge uint32
	code uint32
	/*
	 * Number of initializing bytes remaining to be read
	 * by rcReadInit.
	 */
	initBytesLeft uint32
	/*
	 * Buffer from which we read our input. It can be either
	 * temp.buf or the caller-provided input buffer.
	 */
	in      []byte
	inPos   int
	inLimit int
}

/* Probabilities for a length decoder. */
type lzmaLenDec struct {
	/* Probability of match length being at least 10 */
	choice uint16
	/* Probability of match length being at least 18 */
	choice2 uint16
	/* Probabilities for match lengths 2-9 */
	low [posStatesMax][lenLowSymbols]uint16
	/* Probabilities for match lengths 10-17 */
	mid [posStatesMax][lenMidSymbols]uint16
	/* Probabilities for match lengths 18-273 */
	high [lenHighSymbols]uint16
}

type lzmaDec struct {
	/* Distances of latest four matches */
	rep0 uint32
	rep1 uint32
	rep2 uint32
	rep3 uint32
	/* Types of the most recently seen LZMA symbols */
	state lzmaState
	/*
	 * Length of a match. This is updated so that dictRepeat can
	 * be called again to finish repeating the whole match.
	 */
	len uint32
	/*
	 * LZMA properties or related bit masks (number of literal
	 * context bits, a mask derived from the number of literal
	 * position bits, and a mask derived from the number
	 * position bits)
	 */
	lc             uint32
	literalPosMask uint32
	posMask        uint32
	/* If 1, it's a match. Otherwise it's a single 8-bit literal. */
	isMatch [states][posStatesMax]uint16
	/* If 1, it's a repeated match. The distance is one of rep0 .. rep3. */
	isRep [states]uint16
	/*
	 * If 0, distance of a repeated match is rep0.
	 * Otherwise check is_rep1.
	 */
	isRep0 [states]uint16
	/*
	 * If 0, distance of a repeated match is rep1.
	 * Otherwise check is_rep2.
	 */
	isRep1 [states]uint16
	/* If 0, distance of a repeated match is rep2. Otherwise it is rep3. */
	isRep2 [states]uint16
	/*
	 * If 1, the repeated match has length of one byte. Otherwise
	 * the length is decoded from rep_len_decoder.
	 */
	isRep0Long [states][posStatesMax]uint16
	/*
	 * Probability tree for the highest two bits of the match
	 * distance. There is a separate probability tree for match
	 * lengths of 2 (i.e. MATCH_LEN_MIN), 3, 4, and [5, 273].
	 */
	distSlot [distStates][distSlots]uint16
	/*
	 * Probility trees for additional bits for match distance
	 * when the distance is in the range [4, 127].
	 */
	distSpecial [fullDistances - distModelEnd]uint16
	/*
	 * Probability tree for the lowest four bits of a match
	 * distance that is equal to or greater than 128.
	 */
	distAlign [alignSize]uint16
	/* Length of a normal match */
	matchLenDec lzmaLenDec
	/* Length of a repeated match */
	repLenDec lzmaLenDec
	/* Probabilities of literals */
	literal [literalCodersMax][literalCoderSize]uint16
}

// type of lzma2Dec.sequence
type lzma2Seq int

const (
	seqControl lzma2Seq = iota
	seqUncompressed1
	seqUncompressed2
	seqCompressed0
	seqCompressed1
	seqProperties
	seqLZMAPrepare
	seqLZMARun
	seqCopy
)

type lzma2Dec struct {
	/* Position in xzDecLZMA2Run. */
	sequence lzma2Seq
	/* Next position after decoding the compressed size of the chunk. */
	nextSequence lzma2Seq
	/* Uncompressed size of LZMA chunk (2 MiB at maximum) */
	uncompressed int
	/*
	 * Compressed size of LZMA chunk or compressed/uncompressed
	 * size of uncompressed chunk (64 KiB at maximum)
	 */
	compressed int
	/*
	 * True if dictionary reset is needed. This is false before
	 * the first chunk (LZMA or uncompressed).
	 */
	needDictReset bool
	/*
	 * True if new LZMA properties are needed. This is false
	 * before the first LZMA chunk.
	 */
	needProps bool
}

type xzDecLZMA2 struct {
	/*
	 * The order below is important on x86 to reduce code size and
	 * it shouldn't hurt on other platforms. Everything up to and
	 * including lzma.pos_mask are in the first 128 bytes on x86-32,
	 * which allows using smaller instructions to access those
	 * variables. On x86-64, fewer variables fit into the first 128
	 * bytes, but this is still the best order without sacrificing
	 * the readability by splitting the structures.
	 */
	rc    rcDec
	dict  dictionary
	lzma2 lzma2Dec
	lzma  lzmaDec
	/*
	 * Temporary buffer which holds small number of input bytes between
	 * decoder calls. See lzma2LZMA for details.
	 */
	temp struct {
		buf      []byte // slice buf will be backed by bufArray
		bufArray [3 * lzmaInRequired]byte
	}
}

/**************
 * Dictionary *
 **************/

/*
 * Reset the dictionary state. When in single-call mode, set up the beginning
 * of the dictionary to point to the actual output buffer.
 */
func dictReset(dict *dictionary, b *xzBuf) {
	dict.start = 0
	dict.pos = 0
	dict.limit = 0
	dict.full = 0
}

/* Set dictionary write limit */
func dictLimit(dict *dictionary, outMax int) {
	if dict.end-dict.pos <= uint32(outMax) {
		dict.limit = dict.end
	} else {
		dict.limit = dict.pos + uint32(outMax)
	}
}

/* Return true if at least one byte can be written into the dictionary. */
func dictHasSpace(dict *dictionary) bool {
	return dict.pos < dict.limit
}

/*
 * Get a byte from the dictionary at the given distance. The distance is
 * assumed to valid, or as a special case, zero when the dictionary is
 * still empty. This special case is needed for single-call decoding to
 * avoid writing a '\x00' to the end of the destination buffer.
 */
func dictGet(dict *dictionary, dist uint32) uint32 {
	var offset uint32 = dict.pos - dist - 1
	if dist >= dict.pos {
		offset += dict.end
	}
	if dict.full > 0 {
		return uint32(dict.buf[offset])
	}
	return 0
}

/*
 * Put one byte into the dictionary. It is assumed that there is space for it.
 */
func dictPut(dict *dictionary, byte byte) {
	dict.buf[dict.pos] = byte
	dict.pos++
	if dict.full < dict.pos {
		dict.full = dict.pos
	}
}

/*
 * Repeat given number of bytes from the given distance. If the distance is
 * invalid, false is returned. On success, true is returned and *len is
 * updated to indicate how many bytes were left to be repeated.
 */
func dictRepeat(dict *dictionary, len *uint32, dist uint32) bool {
	var back uint32
	var left uint32
	if dist >= dict.full || dist >= dict.size {
		return false
	}
	left = dict.limit - dict.pos
	if left > *len {
		left = *len
	}
	*len -= left
	back = dict.pos - dist - 1
	if dist >= dict.pos {
		back += dict.end
	}
	for {
		dict.buf[dict.pos] = dict.buf[back]
		dict.pos++
		back++
		if back == dict.end {
			back = 0
		}
		left--
		if !(left > 0) {
			break
		}
	}
	if dict.full < dict.pos {
		dict.full = dict.pos
	}
	return true
}

/* Copy uncompressed data as is from input to dictionary and output buffers. */
func dictUncompressed(dict *dictionary, b *xzBuf, left *int) {
	var copySize int
	for *left > 0 && b.inPos < len(b.in) && b.outPos < len(b.out) {
		copySize = len(b.in) - b.inPos
		if copySize > len(b.out)-b.outPos {
			copySize = len(b.out) - b.outPos
		}
		if copySize > int(dict.end-dict.pos) {
			copySize = int(dict.end - dict.pos)
		}
		if copySize > *left {
			copySize = *left
		}
		*left -= copySize
		copy(dict.buf[dict.pos:], b.in[b.inPos:b.inPos+copySize])
		dict.pos += uint32(copySize)
		if dict.full < dict.pos {
			dict.full = dict.pos
		}
		if dict.pos == dict.end {
			dict.pos = 0
		}
		copy(b.out[b.outPos:], b.in[b.inPos:b.inPos+copySize])
		dict.start = dict.pos
		b.outPos += copySize
		b.inPos += copySize
	}
}

/*
 * Flush pending data from dictionary to b.out. It is assumed that there is
 * enough space in b.out. This is guaranteed because caller uses dictLimit
 * before decoding data into the dictionary.
 */
func dictFlush(dict *dictionary, b *xzBuf) int {
	var copySize int = int(dict.pos - dict.start)
	if dict.pos == dict.end {
		dict.pos = 0
	}
	copy(b.out[b.outPos:], dict.buf[dict.start:dict.start+uint32(copySize)])
	dict.start = dict.pos
	b.outPos += copySize
	return copySize
}

/*****************
 * Range decoder *
 *****************/

/* Reset the range decoder. */
func rcReset(rc *rcDec) {
	rc.rnge = ^uint32(0)
	rc.code = 0
	rc.initBytesLeft = rcInitBytes
}

/*
 * Read the first five initial bytes into rc->code if they haven't been
 * read already. (Yes, the first byte gets completely ignored.)
 */
func rcReadInit(rc *rcDec, b *xzBuf) bool {
	for rc.initBytesLeft > 0 {
		if b.inPos == len(b.in) {
			return false
		}
		rc.code = rc.code<<8 + uint32(b.in[b.inPos])
		b.inPos++
		rc.initBytesLeft--
	}
	return true
}

/* Return true if there may not be enough input for the next decoding loop. */
func rcLimitExceeded(rc *rcDec) bool {
	return rc.inPos > rc.inLimit
}

/*
 * Return true if it is possible (from point of view of range decoder) that
 * we have reached the end of the LZMA chunk.
 */
func rcIsFinished(rc *rcDec) bool {
	return rc.code == 0
}

/* Read the next input byte if needed. */
func rcNormalize(rc *rcDec) {
	if rc.rnge < rcTopValue {
		rc.rnge <<= rcShiftBits
		rc.code = rc.code<<rcShiftBits + uint32(rc.in[rc.inPos])
		rc.inPos++
	}
}

/* Decode one bit. */
func rcBit(rc *rcDec, prob *uint16) bool {
	var bound uint32
	var bit bool
	rcNormalize(rc)
	bound = (rc.rnge >> rcBitModelTotalBits) * uint32(*prob)
	if rc.code < bound {
		rc.rnge = bound
		*prob += (rcBitModelTotal - *prob) >> rcMoveBits
		bit = false
	} else {
		rc.rnge -= bound
		rc.code -= bound
		*prob -= *prob >> rcMoveBits
		bit = true
	}
	return bit
}

/* Decode a bittree starting from the most significant bit. */
func rcBittree(rc *rcDec, probs []uint16, limit uint32) uint32 {
	var symbol uint32 = 1
	for {
		if rcBit(rc, &probs[symbol-1]) {
			symbol = symbol<<1 + 1
		} else {
			symbol <<= 1
		}
		if !(symbol < limit) {
			break
		}
	}
	return symbol
}

/* Decode a bittree starting from the least significant bit. */
func rcBittreeReverse(rc *rcDec, probs []uint16, dest *uint32, limit uint32) {
	var symbol uint32 = 1
	var i uint32 = 0
	for {
		if rcBit(rc, &probs[symbol-1]) {
			symbol = symbol<<1 + 1
			*dest += 1 << i
		} else {
			symbol <<= 1
		}
		i++
		if !(i < limit) {
			break
		}
	}
}

/* Decode direct bits (fixed fifty-fifty probability) */
func rcDirect(rc *rcDec, dest *uint32, limit uint32) {
	var mask uint32
	for {
		rcNormalize(rc)
		rc.rnge >>= 1
		rc.code -= rc.rnge
		mask = 0 - rc.code>>31
		rc.code += rc.rnge & mask
		*dest = *dest<<1 + mask + 1
		limit--
		if !(limit > 0) {
			break
		}
	}
}

/********
 * LZMA *
 ********/

/* Get pointer to literal coder probability array. */
func lzmaLiteralProbs(s *xzDecLZMA2) []uint16 {
	var prevByte uint32 = dictGet(&s.dict, 0)
	var low uint32 = prevByte >> (8 - s.lzma.lc)
	var high uint32 = (s.dict.pos & s.lzma.literalPosMask) << s.lzma.lc
	return s.lzma.literal[low+high][:]
}

/* Decode a literal (one 8-bit byte) */
func lzmaLiteral(s *xzDecLZMA2) {
	var probs []uint16
	var symbol uint32
	var matchByte uint32
	var matchBit uint32
	var offset uint32
	var i uint32
	probs = lzmaLiteralProbs(s)
	if lzmaStateIsLiteral(s.lzma.state) {
		symbol = rcBittree(&s.rc, probs[1:], 0x100)
	} else {
		symbol = 1
		matchByte = dictGet(&s.dict, s.lzma.rep0) << 1
		offset = 0x100
		for {
			matchBit = matchByte & offset
			matchByte <<= 1
			i = offset + matchBit + symbol
			if rcBit(&s.rc, &probs[i]) {
				symbol = symbol<<1 + 1
				offset &= matchBit
			} else {
				symbol <<= 1
				offset &= ^matchBit
			}
			if !(symbol < 0x100) {
				break
			}
		}
	}
	dictPut(&s.dict, byte(symbol))
	lzmaStateLiteral(&s.lzma.state)
}

/* Decode the length of the match into s.lzma.len. */
func lzmaLen(s *xzDecLZMA2, l *lzmaLenDec, posState uint32) {
	var probs []uint16
	var limit uint32
	switch {
	case !rcBit(&s.rc, &l.choice):
		probs = l.low[posState][:]
		limit = lenLowSymbols
		s.lzma.len = matchLenMin
	case !rcBit(&s.rc, &l.choice2):
		probs = l.mid[posState][:]
		limit = lenMidSymbols
		s.lzma.len = matchLenMin + lenLowSymbols
	default:
		probs = l.high[:]
		limit = lenHighSymbols
		s.lzma.len = matchLenMin + lenLowSymbols + lenMidSymbols
	}
	s.lzma.len += rcBittree(&s.rc, probs[1:], limit) - limit
}

/* Decode a match. The distance will be stored in s.lzma.rep0. */
func lzmaMatch(s *xzDecLZMA2, posState uint32) {
	var probs []uint16
	var distSlot uint32
	var limit uint32
	lzmaStateMatch(&s.lzma.state)
	s.lzma.rep3 = s.lzma.rep2
	s.lzma.rep2 = s.lzma.rep1
	s.lzma.rep1 = s.lzma.rep0
	lzmaLen(s, &s.lzma.matchLenDec, posState)
	probs = s.lzma.distSlot[lzmaGetDistState(s.lzma.len)][:]
	distSlot = rcBittree(&s.rc, probs[1:], distSlots) - distSlots
	if distSlot < distModelStart {
		s.lzma.rep0 = distSlot
	} else {
		limit = distSlot>>1 - 1
		s.lzma.rep0 = 2 + distSlot&1
		if distSlot < distModelEnd {
			s.lzma.rep0 <<= limit
			probs = s.lzma.distSpecial[s.lzma.rep0-distSlot:]
			rcBittreeReverse(&s.rc, probs, &s.lzma.rep0, limit)
		} else {
			rcDirect(&s.rc, &s.lzma.rep0, limit-alignBits)
			s.lzma.rep0 <<= alignBits
			rcBittreeReverse(
				&s.rc, s.lzma.distAlign[1:], &s.lzma.rep0, alignBits)
		}
	}
}

/*
 * Decode a repeated match. The distance is one of the four most recently
 * seen matches. The distance will be stored in s.lzma.rep0.
 */
func lzmaRepMatch(s *xzDecLZMA2, posState uint32) {
	var tmp uint32
	if !rcBit(&s.rc, &s.lzma.isRep0[s.lzma.state]) {
		if !rcBit(&s.rc, &s.lzma.isRep0Long[s.lzma.state][posState]) {
			lzmaStateShortRep(&s.lzma.state)
			s.lzma.len = 1
			return
		}
	} else {
		if !rcBit(&s.rc, &s.lzma.isRep1[s.lzma.state]) {
			tmp = s.lzma.rep1
		} else {
			if !rcBit(&s.rc, &s.lzma.isRep2[s.lzma.state]) {
				tmp = s.lzma.rep2
			} else {
				tmp = s.lzma.rep3
				s.lzma.rep3 = s.lzma.rep2
			}
			s.lzma.rep2 = s.lzma.rep1
		}
		s.lzma.rep1 = s.lzma.rep0
		s.lzma.rep0 = tmp
	}
	lzmaStateLongRep(&s.lzma.state)
	lzmaLen(s, &s.lzma.repLenDec, posState)
}

/* LZMA decoder core */
func lzmaMain(s *xzDecLZMA2) bool {
	var posState uint32
	/*
	 * If the dictionary was reached during the previous call, try to
	 * finish the possibly pending repeat in the dictionary.
	 */
	if dictHasSpace(&s.dict) && s.lzma.len > 0 {
		dictRepeat(&s.dict, &s.lzma.len, s.lzma.rep0)
	}
	/*
	 * Decode more LZMA symbols. One iteration may consume up to
	 * lzmaInRequired - 1 bytes.
	 */
	for dictHasSpace(&s.dict) && !rcLimitExceeded(&s.rc) {
		posState = s.dict.pos & s.lzma.posMask
		if !rcBit(&s.rc, &s.lzma.isMatch[s.lzma.state][posState]) {
			lzmaLiteral(s)
		} else {
			if rcBit(&s.rc, &s.lzma.isRep[s.lzma.state]) {
				lzmaRepMatch(s, posState)
			} else {
				lzmaMatch(s, posState)
			}
			if !dictRepeat(&s.dict, &s.lzma.len, s.lzma.rep0) {
				return false
			}
		}
	}
	/*
	 * Having the range decoder always normalized when we are outside
	 * this function makes it easier to correctly handle end of the chunk.
	 */
	rcNormalize(&s.rc)
	return true
}

/*
 * Reset the LZMA decoder and range decoder state. Dictionary is not reset
 * here, because LZMA state may be reset without resetting the dictionary.
 */
func lzmaReset(s *xzDecLZMA2) {
	s.lzma.state = stateLitLit
	s.lzma.rep0 = 0
	s.lzma.rep1 = 0
	s.lzma.rep2 = 0
	s.lzma.rep3 = 0
	/* All probabilities are initialized to the same value, v */
	v := uint16(rcBitModelTotal / 2)
	s.lzma.matchLenDec.choice = v
	s.lzma.matchLenDec.choice2 = v
	s.lzma.repLenDec.choice = v
	s.lzma.repLenDec.choice2 = v
	for _, m := range [][]uint16{
		s.lzma.isRep[:], s.lzma.isRep0[:], s.lzma.isRep1[:],
		s.lzma.isRep2[:], s.lzma.distSpecial[:], s.lzma.distAlign[:],
		s.lzma.matchLenDec.high[:], s.lzma.repLenDec.high[:],
	} {
		for j := range m {
			m[j] = v
		}
	}
	for i := range s.lzma.isMatch {
		for j := range s.lzma.isMatch[i] {
			s.lzma.isMatch[i][j] = v
		}
	}
	for i := range s.lzma.isRep0Long {
		for j := range s.lzma.isRep0Long[i] {
			s.lzma.isRep0Long[i][j] = v
		}
	}
	for i := range s.lzma.distSlot {
		for j := range s.lzma.distSlot[i] {
			s.lzma.distSlot[i][j] = v
		}
	}
	for i := range s.lzma.literal {
		for j := range s.lzma.literal[i] {
			s.lzma.literal[i][j] = v
		}
	}
	for i := range s.lzma.matchLenDec.low {
		for j := range s.lzma.matchLenDec.low[i] {
			s.lzma.matchLenDec.low[i][j] = v
		}
	}
	for i := range s.lzma.matchLenDec.mid {
		for j := range s.lzma.matchLenDec.mid[i] {
			s.lzma.matchLenDec.mid[i][j] = v
		}
	}
	for i := range s.lzma.repLenDec.low {
		for j := range s.lzma.repLenDec.low[i] {
			s.lzma.repLenDec.low[i][j] = v
		}
	}
	for i := range s.lzma.repLenDec.mid {
		for j := range s.lzma.repLenDec.mid[i] {
			s.lzma.repLenDec.mid[i][j] = v
		}
	}
	rcReset(&s.rc)
}

/*
 * Decode and validate LZMA properties (lc/lp/pb) and calculate the bit masks
 * from the decoded lp and pb values. On success, the LZMA decoder state is
 * reset and true is returned.
 */
func lzmaProps(s *xzDecLZMA2, props byte) bool {
	if props > (4*5+4)*9+8 {
		return false
	}
	s.lzma.posMask = 0
	for props >= 9*5 {
		props -= 9 * 5
		s.lzma.posMask++
	}
	s.lzma.posMask = 1<<s.lzma.posMask - 1
	s.lzma.literalPosMask = 0
	for props >= 9 {
		props -= 9
		s.lzma.literalPosMask++
	}
	s.lzma.lc = uint32(props)
	if s.lzma.lc+s.lzma.literalPosMask > 4 {
		return false
	}
	s.lzma.literalPosMask = 1<<s.lzma.literalPosMask - 1
	lzmaReset(s)
	return true
}

/*********
 * LZMA2 *
 *********/

/*
 * The LZMA decoder assumes that if the input limit (s.rc.inLimit) hasn't
 * been exceeded, it is safe to read up to lzmaInRequired bytes. This
 * wrapper function takes care of making the LZMA decoder's assumption safe.
 *
 * As long as there is plenty of input left to be decoded in the current LZMA
 * chunk, we decode directly from the caller-supplied input buffer until
 * there's lzmaInRequired bytes left. Those remaining bytes are copied into
 * s.temp.buf, which (hopefully) gets filled on the next call to this
 * function. We decode a few bytes from the temporary buffer so that we can
 * continue decoding from the caller-supplied input buffer again.
 */
func lzma2LZMA(s *xzDecLZMA2, b *xzBuf) bool {
	var inAvail int
	var tmp int
	inAvail = len(b.in) - b.inPos
	if len(s.temp.buf) > 0 || s.lzma2.compressed == 0 {
		tmp = 2*lzmaInRequired - len(s.temp.buf)
		if tmp > s.lzma2.compressed-len(s.temp.buf) {
			tmp = s.lzma2.compressed - len(s.temp.buf)
		}
		if tmp > inAvail {
			tmp = inAvail
		}
		copy(s.temp.bufArray[len(s.temp.buf):], b.in[b.inPos:b.inPos+tmp])
		switch {
		case len(s.temp.buf)+tmp == s.lzma2.compressed:
			for i := len(s.temp.buf) + tmp; i < len(s.temp.bufArray); i++ {
				s.temp.bufArray[i] = 0
			}
			s.rc.inLimit = len(s.temp.buf) + tmp
		case len(s.temp.buf)+tmp < lzmaInRequired:
			s.temp.buf = s.temp.bufArray[:len(s.temp.buf)+tmp]
			b.inPos += tmp
			return true
		default:
			s.rc.inLimit = len(s.temp.buf) + tmp - lzmaInRequired
		}
		s.rc.in = s.temp.bufArray[:]
		s.rc.inPos = 0
		if !lzmaMain(s) || s.rc.inPos > len(s.temp.buf)+tmp {
			return false
		}
		s.lzma2.compressed -= s.rc.inPos
		if s.rc.inPos < len(s.temp.buf) {
			copy(s.temp.buf, s.temp.buf[s.rc.inPos:])
			s.temp.buf = s.temp.buf[:len(s.temp.buf)-s.rc.inPos]
			return true
		}
		b.inPos += s.rc.inPos - len(s.temp.buf)
		s.temp.buf = nil
	}
	inAvail = len(b.in) - b.inPos
	if inAvail >= lzmaInRequired {
		s.rc.in = b.in
		s.rc.inPos = b.inPos
		if inAvail >= s.lzma2.compressed+lzmaInRequired {
			s.rc.inLimit = b.inPos + s.lzma2.compressed
		} else {
			s.rc.inLimit = len(b.in) - lzmaInRequired
		}
		if !lzmaMain(s) {
			return false
		}
		inAvail = s.rc.inPos - b.inPos
		if inAvail > s.lzma2.compressed {
			return false
		}
		s.lzma2.compressed -= inAvail
		b.inPos = s.rc.inPos
	}
	inAvail = len(b.in) - b.inPos
	if inAvail < lzmaInRequired {
		if inAvail > s.lzma2.compressed {
			inAvail = s.lzma2.compressed
		}
		s.temp.buf = s.temp.bufArray[:inAvail]
		copy(s.temp.buf, b.in[b.inPos:])
		b.inPos += inAvail
	}
	return true
}

/*
 * Take care of the LZMA2 control layer, and forward the job of actual LZMA
 * decoding or copying of uncompressed chunks to other functions.
 */
func xzDecLZMA2Run(s *xzDecLZMA2, b *xzBuf) xzRet {
	var tmp int
	for b.inPos < len(b.in) || s.lzma2.sequence == seqLZMARun {
		switch s.lzma2.sequence {
		case seqControl:
			/*
			 * LZMA2 control byte
			 *
			 * Exact values:
			 *   0x00   End marker
			 *   0x01   Dictionary reset followed by
			 *          an uncompressed chunk
			 *   0x02   Uncompressed chunk (no dictionary reset)
			 *
			 * Highest three bits (s.control & 0xE0):
			 *   0xE0   Dictionary reset, new properties and state
			 *          reset, followed by LZMA compressed chunk
			 *   0xC0   New properties and state reset, followed
			 *          by LZMA compressed chunk (no dictionary
			 *          reset)
			 *   0xA0   State reset using old properties,
			 *          followed by LZMA compressed chunk (no
			 *          dictionary reset)
			 *   0x80   LZMA chunk (no dictionary or state reset)
			 *
			 * For LZMA compressed chunks, the lowest five bits
			 * (s.control & 1F) are the highest bits of the
			 * uncompressed size (bits 16-20).
			 *
			 * A new LZMA2 stream must begin with a dictionary
			 * reset. The first LZMA chunk must set new
			 * properties and reset the LZMA state.
			 *
			 * Values that don't match anything described above
			 * are invalid and we return xzDataError.
			 */
			tmp = int(b.in[b.inPos])
			b.inPos++
			if tmp == 0x00 {
				return xzStreamEnd
			}
			switch {
			case tmp >= 0xe0 || tmp == 0x01:
				s.lzma2.needProps = true
				s.lzma2.needDictReset = false
				dictReset(&s.dict, b)
			case s.lzma2.needDictReset:
				return xzDataError
			}
			if tmp >= 0x80 {
				s.lzma2.uncompressed = (tmp & 0x1f) << 16
				s.lzma2.sequence = seqUncompressed1
				switch {
				case tmp >= 0xc0:
					/*
					 * When there are new properties,
					 * state reset is done at
					 * seqProperties.
					 */
					s.lzma2.needProps = false
					s.lzma2.nextSequence = seqProperties
				case s.lzma2.needProps:
					return xzDataError
				default:
					s.lzma2.nextSequence = seqLZMAPrepare
					if tmp >= 0xa0 {
						lzmaReset(s)
					}
				}
			} else {
				if tmp > 0x02 {
					return xzDataError
				}
				s.lzma2.sequence = seqCompressed0
				s.lzma2.nextSequence = seqCopy
			}
		case seqUncompressed1:
			s.lzma2.uncompressed += int(b.in[b.inPos]) << 8
			b.inPos++
			s.lzma2.sequence = seqUncompressed2
		case seqUncompressed2:
			s.lzma2.uncompressed += int(b.in[b.inPos]) + 1
			b.inPos++
			s.lzma2.sequence = seqCompressed0
		case seqCompressed0:
			s.lzma2.compressed += int(b.in[b.inPos]) << 8
			b.inPos++
			s.lzma2.sequence = seqCompressed1
		case seqCompressed1:
			s.lzma2.compressed += int(b.in[b.inPos]) + 1
			b.inPos++
			s.lzma2.sequence = s.lzma2.nextSequence
		case seqProperties:
			if !lzmaProps(s, b.in[b.inPos]) {
				return xzDataError
			}
			b.inPos++
			s.lzma2.sequence = seqLZMAPrepare
			fallthrough
		case seqLZMAPrepare:
			if s.lzma2.compressed < rcInitBytes {
				return xzDataError
			}
			if !rcReadInit(&s.rc, b) {
				return xzOK
			}
			s.lzma2.compressed -= rcInitBytes
			s.lzma2.sequence = seqLZMARun
			fallthrough
		case seqLZMARun:
			/*
			 * Set dictionary limit to indicate how much we want
			 * to be encoded at maximum. Decode new data into the
			 * dictionary. Flush the new data from dictionary to
			 * b.out. Check if we finished decoding this chunk.
			 * In case the dictionary got full but we didn't fill
			 * the output buffer yet, we may run this loop
			 * multiple times without changing s.lzma2.sequence.
			 */
			outMax := len(b.out) - b.outPos
			if outMax > s.lzma2.uncompressed {
				outMax = s.lzma2.uncompressed
			}
			dictLimit(&s.dict, outMax)
			if !lzma2LZMA(s, b) {
				return xzDataError
			}
			s.lzma2.uncompressed -= dictFlush(&s.dict, b)
			switch {
			case s.lzma2.uncompressed == 0:
				if s.lzma2.compressed > 0 || s.lzma.len > 0 ||
					!rcIsFinished(&s.rc) {
					return xzDataError
				}
				rcReset(&s.rc)
				s.lzma2.sequence = seqControl
			case b.outPos == len(b.out) ||
				b.inPos == len(b.in) &&
					len(s.temp.buf) < s.lzma2.compressed:
				return xzOK
			}
		case seqCopy:
			dictUncompressed(&s.dict, b, &s.lzma2.compressed)
			if s.lzma2.compressed > 0 {
				return xzOK
			}
			s.lzma2.sequence = seqControl
		}
	}
	return xzOK
}

/*
 * Allocate memory for LZMA2 decoder. xzDecLZMA2Reset must be used
 * before calling xzDecLZMA2Run.
 */
func xzDecLZMA2Create(dictMax uint32) *xzDecLZMA2 {
	s := new(xzDecLZMA2)
	s.dict.sizeMax = dictMax
	return s
}

/*
 * Decode the LZMA2 properties (one byte) and reset the decoder. Return
 * xzOK on success, xzMemlimitError if the preallocated dictionary is not
 * big enough, and xzOptionsError if props indicates something that this
 * decoder doesn't support.
 */
func xzDecLZMA2Reset(s *xzDecLZMA2, props byte) xzRet {
	if props > 40 {
		return xzOptionsError // Bigger than 4 GiB
	}
	if props == 40 {
		s.dict.size = ^uint32(0)
	} else {
		s.dict.size = uint32(2 + props&1)
		s.dict.size <<= props>>1 + 11
	}
	if s.dict.size > s.dict.sizeMax {
		return xzMemlimitError
	}
	s.dict.end = s.dict.size
	if len(s.dict.buf) < int(s.dict.size) {
		s.dict.buf = make([]byte, s.dict.size)
	}
	s.lzma.len = 0
	s.lzma2.sequence = seqControl
	s.lzma2.compressed = 0
	s.lzma2.uncompressed = 0
	s.lzma2.needDictReset = true
	s.temp.buf = nil
	return xzOK
}
