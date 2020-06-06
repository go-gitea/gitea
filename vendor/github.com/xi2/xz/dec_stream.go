/*
 * .xz Stream decoder
 *
 * Author: Lasse Collin <lasse.collin@tukaani.org>
 *
 * Translation to Go: Michael Cross <https://github.com/xi2>
 *
 * This file has been put into the public domain.
 * You can do whatever you want with this file.
 */

package xz

import (
	"bytes"
	"crypto/sha256"
	"hash"
	"hash/crc32"
	"hash/crc64"
)

/* from linux/lib/xz/xz_stream.h **************************************/

/*
 * See the .xz file format specification at
 * http://tukaani.org/xz/xz-file-format.txt
 * to understand the container format.
 */
const (
	streamHeaderSize = 12
	headerMagic      = "\xfd7zXZ\x00"
	footerMagic      = "YZ"
)

/*
 * Variable-length integer can hold a 63-bit unsigned integer or a special
 * value indicating that the value is unknown.
 */
type vliType uint64

const (
	vliUnknown vliType = ^vliType(0)
	/* Maximum encoded size of a VLI */
	vliBytesMax = 8 * 8 / 7 // (Sizeof(vliType) * 8 / 7)
)

/* from linux/lib/xz/xz_dec_stream.c **********************************/

/* Hash used to validate the Index field */
type xzDecHash struct {
	unpadded     vliType
	uncompressed vliType
	sha256       hash.Hash
}

// type of xzDec.sequence
type xzDecSeq int

const (
	seqStreamHeader xzDecSeq = iota
	seqBlockStart
	seqBlockHeader
	seqBlockUncompress
	seqBlockPadding
	seqBlockCheck
	seqIndex
	seqIndexPadding
	seqIndexCRC32
	seqStreamFooter
)

// type of xzDec.index.sequence
type xzDecIndexSeq int

const (
	seqIndexCount xzDecIndexSeq = iota
	seqIndexUnpadded
	seqIndexUncompressed
)

/**
 * xzDec - Opaque type to hold the XZ decoder state
 */
type xzDec struct {
	/* Position in decMain */
	sequence xzDecSeq
	/* Position in variable-length integers and Check fields */
	pos int
	/* Variable-length integer decoded by decVLI */
	vli vliType
	/* Saved inPos and outPos */
	inStart  int
	outStart int
	/* CRC32 checksum hash used in Index */
	crc32 hash.Hash
	/* Hashes used in Blocks */
	checkCRC32  hash.Hash
	checkCRC64  hash.Hash
	checkSHA256 hash.Hash
	/* for checkTypes CRC32/CRC64/SHA256, check is one of the above 3 hashes */
	check hash.Hash
	/* Embedded stream header struct containing CheckType */
	*Header
	/*
	 * True if the next call to xzDecRun is allowed to return
	 * xzBufError.
	 */
	allowBufError bool
	/* Information stored in Block Header */
	blockHeader struct {
		/*
		 * Value stored in the Compressed Size field, or
		 * vliUnknown if Compressed Size is not present.
		 */
		compressed vliType
		/*
		 * Value stored in the Uncompressed Size field, or
		 * vliUnknown if Uncompressed Size is not present.
		 */
		uncompressed vliType
		/* Size of the Block Header field */
		size int
	}
	/* Information collected when decoding Blocks */
	block struct {
		/* Observed compressed size of the current Block */
		compressed vliType
		/* Observed uncompressed size of the current Block */
		uncompressed vliType
		/* Number of Blocks decoded so far */
		count vliType
		/*
		 * Hash calculated from the Block sizes. This is used to
		 * validate the Index field.
		 */
		hash xzDecHash
	}
	/* Variables needed when verifying the Index field */
	index struct {
		/* Position in decIndex */
		sequence xzDecIndexSeq
		/* Size of the Index in bytes */
		size vliType
		/* Number of Records (matches block.count in valid files) */
		count vliType
		/*
		 * Hash calculated from the Records (matches block.hash in
		 * valid files).
		 */
		hash xzDecHash
	}
	/*
	 * Temporary buffer needed to hold Stream Header, Block Header,
	 * and Stream Footer. The Block Header is the biggest (1 KiB)
	 * so we reserve space according to that. bufArray has to be aligned
	 * to a multiple of four bytes; the variables before it
	 * should guarantee this.
	 */
	temp struct {
		pos      int
		buf      []byte // slice buf will be backed by bufArray
		bufArray [1024]byte
	}
	// chain is the function (or to be more precise, closure) which
	// does the decompression and will call into the lzma2 and other
	// filter code as needed. It is constructed by decBlockHeader
	chain func(b *xzBuf) xzRet
	// lzma2 holds the state of the last filter (which must be LZMA2)
	lzma2 *xzDecLZMA2
	// pointers to allocated BCJ/Delta filters
	bcjs   []*xzDecBCJ
	deltas []*xzDecDelta
	// number of currently in use BCJ/Delta filters from the above
	bcjsUsed   int
	deltasUsed int
}

/* Sizes of the Check field with different Check IDs */
var checkSizes = [...]byte{
	0,
	4, 4, 4,
	8, 8, 8,
	16, 16, 16,
	32, 32, 32,
	64, 64, 64,
}

/*
 * Fill s.temp by copying data starting from b.in[b.inPos]. Caller
 * must have set s.temp.pos to indicate how much data we are supposed
 * to copy into s.temp.buf. Return true once s.temp.pos has reached
 * len(s.temp.buf).
 */
func fillTemp(s *xzDec, b *xzBuf) bool {
	copySize := len(b.in) - b.inPos
	tempRemaining := len(s.temp.buf) - s.temp.pos
	if copySize > tempRemaining {
		copySize = tempRemaining
	}
	copy(s.temp.buf[s.temp.pos:], b.in[b.inPos:])
	b.inPos += copySize
	s.temp.pos += copySize
	if s.temp.pos == len(s.temp.buf) {
		s.temp.pos = 0
		return true
	}
	return false
}

/* Decode a variable-length integer (little-endian base-128 encoding) */
func decVLI(s *xzDec, in []byte, inPos *int) xzRet {
	var byte byte
	if s.pos == 0 {
		s.vli = 0
	}
	for *inPos < len(in) {
		byte = in[*inPos]
		*inPos++
		s.vli |= vliType(byte&0x7f) << uint(s.pos)
		if byte&0x80 == 0 {
			/* Don't allow non-minimal encodings. */
			if byte == 0 && s.pos != 0 {
				return xzDataError
			}
			s.pos = 0
			return xzStreamEnd
		}
		s.pos += 7
		if s.pos == 7*vliBytesMax {
			return xzDataError
		}
	}
	return xzOK
}

/*
 * Decode the Compressed Data field from a Block. Update and validate
 * the observed compressed and uncompressed sizes of the Block so that
 * they don't exceed the values possibly stored in the Block Header
 * (validation assumes that no integer overflow occurs, since vliType
 * is uint64). Update s.check if presence of the CRC32/CRC64/SHA256
 * field was indicated in Stream Header.
 *
 * Once the decoding is finished, validate that the observed sizes match
 * the sizes possibly stored in the Block Header. Update the hash and
 * Block count, which are later used to validate the Index field.
 */
func decBlock(s *xzDec, b *xzBuf) xzRet {
	var ret xzRet
	s.inStart = b.inPos
	s.outStart = b.outPos
	ret = s.chain(b)
	s.block.compressed += vliType(b.inPos - s.inStart)
	s.block.uncompressed += vliType(b.outPos - s.outStart)
	/*
	 * There is no need to separately check for vliUnknown since
	 * the observed sizes are always smaller than vliUnknown.
	 */
	if s.block.compressed > s.blockHeader.compressed ||
		s.block.uncompressed > s.blockHeader.uncompressed {
		return xzDataError
	}
	switch s.CheckType {
	case CheckCRC32, CheckCRC64, CheckSHA256:
		_, _ = s.check.Write(b.out[s.outStart:b.outPos])
	}
	if ret == xzStreamEnd {
		if s.blockHeader.compressed != vliUnknown &&
			s.blockHeader.compressed != s.block.compressed {
			return xzDataError
		}
		if s.blockHeader.uncompressed != vliUnknown &&
			s.blockHeader.uncompressed != s.block.uncompressed {
			return xzDataError
		}
		s.block.hash.unpadded +=
			vliType(s.blockHeader.size) + s.block.compressed
		s.block.hash.unpadded += vliType(checkSizes[s.CheckType])
		s.block.hash.uncompressed += s.block.uncompressed
		var buf [2 * 8]byte // 2*Sizeof(vliType)
		putLE64(uint64(s.block.hash.unpadded), buf[:])
		putLE64(uint64(s.block.hash.uncompressed), buf[8:])
		_, _ = s.block.hash.sha256.Write(buf[:])
		s.block.count++
	}
	return ret
}

/* Update the Index size and the CRC32 hash. */
func indexUpdate(s *xzDec, b *xzBuf) {
	inUsed := b.inPos - s.inStart
	s.index.size += vliType(inUsed)
	_, _ = s.crc32.Write(b.in[s.inStart : s.inStart+inUsed])
}

/*
 * Decode the Number of Records, Unpadded Size, and Uncompressed Size
 * fields from the Index field. That is, Index Padding and CRC32 are not
 * decoded by this function.
 *
 * This can return xzOK (more input needed), xzStreamEnd (everything
 * successfully decoded), or xzDataError (input is corrupt).
 */
func decIndex(s *xzDec, b *xzBuf) xzRet {
	var ret xzRet
	for {
		ret = decVLI(s, b.in, &b.inPos)
		if ret != xzStreamEnd {
			indexUpdate(s, b)
			return ret
		}
		switch s.index.sequence {
		case seqIndexCount:
			s.index.count = s.vli
			/*
			 * Validate that the Number of Records field
			 * indicates the same number of Records as
			 * there were Blocks in the Stream.
			 */
			if s.index.count != s.block.count {
				return xzDataError
			}
			s.index.sequence = seqIndexUnpadded
		case seqIndexUnpadded:
			s.index.hash.unpadded += s.vli
			s.index.sequence = seqIndexUncompressed
		case seqIndexUncompressed:
			s.index.hash.uncompressed += s.vli
			var buf [2 * 8]byte // 2*Sizeof(vliType)
			putLE64(uint64(s.index.hash.unpadded), buf[:])
			putLE64(uint64(s.index.hash.uncompressed), buf[8:])
			_, _ = s.index.hash.sha256.Write(buf[:])
			s.index.count--
			s.index.sequence = seqIndexUnpadded
		}
		if !(s.index.count > 0) {
			break
		}
	}
	return xzStreamEnd
}

/*
 * Validate that the next 4 bytes match s.crc32.Sum(nil). s.pos must
 * be zero when starting to validate the first byte.
 */
func crcValidate(s *xzDec, b *xzBuf) xzRet {
	sum := s.crc32.Sum(nil)
	// CRC32 - reverse slice
	sum[0], sum[1], sum[2], sum[3] = sum[3], sum[2], sum[1], sum[0]
	for {
		if b.inPos == len(b.in) {
			return xzOK
		}
		if sum[s.pos] != b.in[b.inPos] {
			return xzDataError
		}
		b.inPos++
		s.pos++
		if !(s.pos < 4) {
			break
		}
	}
	s.crc32.Reset()
	s.pos = 0
	return xzStreamEnd
}

/*
 * Validate that the next 4/8/32 bytes match s.check.Sum(nil). s.pos
 * must be zero when starting to validate the first byte.
 */
func checkValidate(s *xzDec, b *xzBuf) xzRet {
	sum := s.check.Sum(nil)
	if s.CheckType == CheckCRC32 || s.CheckType == CheckCRC64 {
		// CRC32/64 - reverse slice
		for i, j := 0, len(sum)-1; i < j; i, j = i+1, j-1 {
			sum[i], sum[j] = sum[j], sum[i]
		}
	}
	for {
		if b.inPos == len(b.in) {
			return xzOK
		}
		if sum[s.pos] != b.in[b.inPos] {
			return xzDataError
		}
		b.inPos++
		s.pos++
		if !(s.pos < len(sum)) {
			break
		}
	}
	s.check.Reset()
	s.pos = 0
	return xzStreamEnd
}

/*
 * Skip over the Check field when the Check ID is not supported.
 * Returns true once the whole Check field has been skipped over.
 */
func checkSkip(s *xzDec, b *xzBuf) bool {
	for s.pos < int(checkSizes[s.CheckType]) {
		if b.inPos == len(b.in) {
			return false
		}
		b.inPos++
		s.pos++
	}
	s.pos = 0
	return true
}

/* polynomial table used in decStreamHeader below */
var xzCRC64Table = crc64.MakeTable(crc64.ECMA)

/* Decode the Stream Header field (the first 12 bytes of the .xz Stream). */
func decStreamHeader(s *xzDec) xzRet {
	if string(s.temp.buf[:len(headerMagic)]) != headerMagic {
		return xzFormatError
	}
	if crc32.ChecksumIEEE(s.temp.buf[len(headerMagic):len(headerMagic)+2]) !=
		getLE32(s.temp.buf[len(headerMagic)+2:]) {
		return xzDataError
	}
	if s.temp.buf[len(headerMagic)] != 0 {
		return xzOptionsError
	}
	/*
	 * Of integrity checks, we support none (Check ID = 0),
	 * CRC32 (Check ID = 1), CRC64 (Check ID = 4) and SHA256 (Check ID = 10)
	 * However, we will accept other check types too, but then the check
	 * won't be verified and a warning (xzUnsupportedCheck) will be given.
	 */
	s.CheckType = CheckID(s.temp.buf[len(headerMagic)+1])
	if s.CheckType > checkMax {
		return xzOptionsError
	}
	switch s.CheckType {
	case CheckNone:
		// CheckNone: no action needed
	case CheckCRC32:
		if s.checkCRC32 == nil {
			s.checkCRC32 = crc32.NewIEEE()
		} else {
			s.checkCRC32.Reset()
		}
		s.check = s.checkCRC32
	case CheckCRC64:
		if s.checkCRC64 == nil {
			s.checkCRC64 = crc64.New(xzCRC64Table)
		} else {
			s.checkCRC64.Reset()
		}
		s.check = s.checkCRC64
	case CheckSHA256:
		if s.checkSHA256 == nil {
			s.checkSHA256 = sha256.New()
		} else {
			s.checkSHA256.Reset()
		}
		s.check = s.checkSHA256
	default:
		return xzUnsupportedCheck
	}
	return xzOK
}

/* Decode the Stream Footer field (the last 12 bytes of the .xz Stream) */
func decStreamFooter(s *xzDec) xzRet {
	if string(s.temp.buf[10:10+len(footerMagic)]) != footerMagic {
		return xzDataError
	}
	if crc32.ChecksumIEEE(s.temp.buf[4:10]) != getLE32(s.temp.buf) {
		return xzDataError
	}
	/*
	 * Validate Backward Size. Note that we never added the size of the
	 * Index CRC32 field to s->index.size, thus we use s->index.size / 4
	 * instead of s->index.size / 4 - 1.
	 */
	if s.index.size>>2 != vliType(getLE32(s.temp.buf[4:])) {
		return xzDataError
	}
	if s.temp.buf[8] != 0 || CheckID(s.temp.buf[9]) != s.CheckType {
		return xzDataError
	}
	/*
	 * Use xzStreamEnd instead of xzOK to be more convenient
	 * for the caller.
	 */
	return xzStreamEnd
}

/* Decode the Block Header and initialize the filter chain. */
func decBlockHeader(s *xzDec) xzRet {
	var ret xzRet
	/*
	 * Validate the CRC32. We know that the temp buffer is at least
	 * eight bytes so this is safe.
	 */
	crc := getLE32(s.temp.buf[len(s.temp.buf)-4:])
	s.temp.buf = s.temp.buf[:len(s.temp.buf)-4]
	if crc32.ChecksumIEEE(s.temp.buf) != crc {
		return xzDataError
	}
	s.temp.pos = 2
	/*
	 * Catch unsupported Block Flags.
	 */
	if s.temp.buf[1]&0x3C != 0 {
		return xzOptionsError
	}
	/* Compressed Size */
	if s.temp.buf[1]&0x40 != 0 {
		if decVLI(s, s.temp.buf, &s.temp.pos) != xzStreamEnd {
			return xzDataError
		}
		if s.vli >= 1<<63-8 {
			// the whole block must stay smaller than 2^63 bytes
			// the block header cannot be smaller than 8 bytes
			return xzDataError
		}
		if s.vli == 0 {
			// compressed size must be non-zero
			return xzDataError
		}
		s.blockHeader.compressed = s.vli
	} else {
		s.blockHeader.compressed = vliUnknown
	}
	/* Uncompressed Size */
	if s.temp.buf[1]&0x80 != 0 {
		if decVLI(s, s.temp.buf, &s.temp.pos) != xzStreamEnd {
			return xzDataError
		}
		s.blockHeader.uncompressed = s.vli
	} else {
		s.blockHeader.uncompressed = vliUnknown
	}
	// get total number of filters (1-4)
	filterTotal := int(s.temp.buf[1]&0x03) + 1
	// slice to hold decoded filters
	filterList := make([]struct {
		id    xzFilterID
		props uint32
	}, filterTotal)
	// decode the non-last filters which cannot be LZMA2
	for i := 0; i < filterTotal-1; i++ {
		/* Valid Filter Flags always take at least two bytes. */
		if len(s.temp.buf)-s.temp.pos < 2 {
			return xzDataError
		}
		s.temp.pos += 2
		switch id := xzFilterID(s.temp.buf[s.temp.pos-2]); id {
		case idDelta:
			// delta filter
			if s.temp.buf[s.temp.pos-1] != 0x01 {
				return xzOptionsError
			}
			/* Filter Properties contains distance - 1 */
			if len(s.temp.buf)-s.temp.pos < 1 {
				return xzDataError
			}
			props := uint32(s.temp.buf[s.temp.pos])
			s.temp.pos++
			filterList[i] = struct {
				id    xzFilterID
				props uint32
			}{id: id, props: props}
		case idBCJX86, idBCJPowerPC, idBCJIA64,
			idBCJARM, idBCJARMThumb, idBCJSPARC:
			// bcj filter
			var props uint32
			switch s.temp.buf[s.temp.pos-1] {
			case 0x00:
				props = 0
			case 0x04:
				if len(s.temp.buf)-s.temp.pos < 4 {
					return xzDataError
				}
				props = getLE32(s.temp.buf[s.temp.pos:])
				s.temp.pos += 4
			default:
				return xzOptionsError
			}
			filterList[i] = struct {
				id    xzFilterID
				props uint32
			}{id: id, props: props}
		default:
			return xzOptionsError
		}
	}
	/*
	 * decode the last filter which must be LZMA2
	 */
	if len(s.temp.buf)-s.temp.pos < 2 {
		return xzDataError
	}
	/* Filter ID = LZMA2 */
	if xzFilterID(s.temp.buf[s.temp.pos]) != idLZMA2 {
		return xzOptionsError
	}
	s.temp.pos++
	/* Size of Properties = 1-byte Filter Properties */
	if s.temp.buf[s.temp.pos] != 0x01 {
		return xzOptionsError
	}
	s.temp.pos++
	/* Filter Properties contains LZMA2 dictionary size. */
	if len(s.temp.buf)-s.temp.pos < 1 {
		return xzDataError
	}
	props := uint32(s.temp.buf[s.temp.pos])
	s.temp.pos++
	filterList[filterTotal-1] = struct {
		id    xzFilterID
		props uint32
	}{id: idLZMA2, props: props}
	/*
	 * Process the filter list and create s.chain, going from last
	 * filter (LZMA2) to first filter
	 *
	 * First, LZMA2.
	 */
	ret = xzDecLZMA2Reset(s.lzma2, byte(filterList[filterTotal-1].props))
	if ret != xzOK {
		return ret
	}
	s.chain = func(b *xzBuf) xzRet {
		return xzDecLZMA2Run(s.lzma2, b)
	}
	/*
	 * Now the non-last filters
	 */
	for i := filterTotal - 2; i >= 0; i-- {
		switch id := filterList[i].id; id {
		case idDelta:
			// delta filter
			var delta *xzDecDelta
			if s.deltasUsed < len(s.deltas) {
				delta = s.deltas[s.deltasUsed]
			} else {
				delta = xzDecDeltaCreate()
				s.deltas = append(s.deltas, delta)
			}
			s.deltasUsed++
			ret = xzDecDeltaReset(delta, int(filterList[i].props)+1)
			if ret != xzOK {
				return ret
			}
			chain := s.chain
			s.chain = func(b *xzBuf) xzRet {
				return xzDecDeltaRun(delta, b, chain)
			}
		case idBCJX86, idBCJPowerPC, idBCJIA64,
			idBCJARM, idBCJARMThumb, idBCJSPARC:
			// bcj filter
			var bcj *xzDecBCJ
			if s.bcjsUsed < len(s.bcjs) {
				bcj = s.bcjs[s.bcjsUsed]
			} else {
				bcj = xzDecBCJCreate()
				s.bcjs = append(s.bcjs, bcj)
			}
			s.bcjsUsed++
			ret = xzDecBCJReset(bcj, id, int(filterList[i].props))
			if ret != xzOK {
				return ret
			}
			chain := s.chain
			s.chain = func(b *xzBuf) xzRet {
				return xzDecBCJRun(bcj, b, chain)
			}
		}
	}
	/* The rest must be Header Padding. */
	for s.temp.pos < len(s.temp.buf) {
		if s.temp.buf[s.temp.pos] != 0x00 {
			return xzOptionsError
		}
		s.temp.pos++
	}
	s.temp.pos = 0
	s.block.compressed = 0
	s.block.uncompressed = 0
	return xzOK
}

func decMain(s *xzDec, b *xzBuf) xzRet {
	var ret xzRet
	/*
	 * Store the start position for the case when we are in the middle
	 * of the Index field.
	 */
	s.inStart = b.inPos
	for {
		switch s.sequence {
		case seqStreamHeader:
			/*
			 * Stream Header is copied to s.temp, and then
			 * decoded from there. This way if the caller
			 * gives us only little input at a time, we can
			 * still keep the Stream Header decoding code
			 * simple. Similar approach is used in many places
			 * in this file.
			 */
			if !fillTemp(s, b) {
				return xzOK
			}
			/*
			 * If decStreamHeader returns
			 * xzUnsupportedCheck, it is still possible
			 * to continue decoding. Thus, update s.sequence
			 * before calling decStreamHeader.
			 */
			s.sequence = seqBlockStart
			ret = decStreamHeader(s)
			if ret != xzOK {
				return ret
			}
			fallthrough
		case seqBlockStart:
			/* We need one byte of input to continue. */
			if b.inPos == len(b.in) {
				return xzOK
			}
			/* See if this is the beginning of the Index field. */
			if b.in[b.inPos] == 0 {
				s.inStart = b.inPos
				b.inPos++
				s.sequence = seqIndex
				break
			}
			/*
			 * Calculate the size of the Block Header and
			 * prepare to decode it.
			 */
			s.blockHeader.size = (int(b.in[b.inPos]) + 1) * 4
			s.temp.buf = s.temp.bufArray[:s.blockHeader.size]
			s.temp.pos = 0
			s.sequence = seqBlockHeader
			fallthrough
		case seqBlockHeader:
			if !fillTemp(s, b) {
				return xzOK
			}
			ret = decBlockHeader(s)
			if ret != xzOK {
				return ret
			}
			s.sequence = seqBlockUncompress
			fallthrough
		case seqBlockUncompress:
			ret = decBlock(s, b)
			if ret != xzStreamEnd {
				return ret
			}
			s.sequence = seqBlockPadding
			fallthrough
		case seqBlockPadding:
			/*
			 * Size of Compressed Data + Block Padding
			 * must be a multiple of four. We don't need
			 * s->block.compressed for anything else
			 * anymore, so we use it here to test the size
			 * of the Block Padding field.
			 */
			for s.block.compressed&3 != 0 {
				if b.inPos == len(b.in) {
					return xzOK
				}
				if b.in[b.inPos] != 0 {
					return xzDataError
				}
				b.inPos++
				s.block.compressed++
			}
			s.sequence = seqBlockCheck
			fallthrough
		case seqBlockCheck:
			switch s.CheckType {
			case CheckCRC32, CheckCRC64, CheckSHA256:
				ret = checkValidate(s, b)
				if ret != xzStreamEnd {
					return ret
				}
			default:
				if !checkSkip(s, b) {
					return xzOK
				}
			}
			s.sequence = seqBlockStart
		case seqIndex:
			ret = decIndex(s, b)
			if ret != xzStreamEnd {
				return ret
			}
			s.sequence = seqIndexPadding
			fallthrough
		case seqIndexPadding:
			for (s.index.size+vliType(b.inPos-s.inStart))&3 != 0 {
				if b.inPos == len(b.in) {
					indexUpdate(s, b)
					return xzOK
				}
				if b.in[b.inPos] != 0 {
					return xzDataError
				}
				b.inPos++
			}
			/* Finish the CRC32 value and Index size. */
			indexUpdate(s, b)
			/* Compare the hashes to validate the Index field. */
			if !bytes.Equal(
				s.block.hash.sha256.Sum(nil), s.index.hash.sha256.Sum(nil)) {
				return xzDataError
			}
			s.sequence = seqIndexCRC32
			fallthrough
		case seqIndexCRC32:
			ret = crcValidate(s, b)
			if ret != xzStreamEnd {
				return ret
			}
			s.temp.buf = s.temp.bufArray[:streamHeaderSize]
			s.sequence = seqStreamFooter
			fallthrough
		case seqStreamFooter:
			if !fillTemp(s, b) {
				return xzOK
			}
			return decStreamFooter(s)
		}
	}
	/* Never reached */
}

/**
 * xzDecRun - Run the XZ decoder
 * @s:         Decoder state allocated using xzDecInit
 * @b:         Input and output buffers
 *
 * See xzRet for details of return values.
 *
 * xzDecRun is a wrapper for decMain to handle some special cases.
 *
 * We must return xzBufError when it seems clear that we are not
 * going to make any progress anymore. This is to prevent the caller
 * from calling us infinitely when the input file is truncated or
 * otherwise corrupt. Since zlib-style API allows that the caller
 * fills the input buffer only when the decoder doesn't produce any
 * new output, we have to be careful to avoid returning xzBufError
 * too easily: xzBufError is returned only after the second
 * consecutive call to xzDecRun that makes no progress.
 */
func xzDecRun(s *xzDec, b *xzBuf) xzRet {
	inStart := b.inPos
	outStart := b.outPos
	ret := decMain(s, b)
	if ret == xzOK && inStart == b.inPos && outStart == b.outPos {
		if s.allowBufError {
			ret = xzBufError
		}
		s.allowBufError = true
	} else {
		s.allowBufError = false
	}
	return ret
}

/**
 * xzDecInit - Allocate and initialize a XZ decoder state
 * @dictMax:    Maximum size of the LZMA2 dictionary (history buffer) for
 *              decoding. LZMA2 dictionary is always 2^n bytes
 *              or 2^n + 2^(n-1) bytes (the latter sizes are less common
 *              in practice), so other values for dictMax don't make sense.
 *
 * dictMax specifies the maximum allowed dictionary size that xzDecRun
 * may allocate once it has parsed the dictionary size from the stream
 * headers. This way excessive allocations can be avoided while still
 * limiting the maximum memory usage to a sane value to prevent running the
 * system out of memory when decompressing streams from untrusted sources.
 *
 * xzDecInit returns a pointer to an xzDec, which is ready to be used with
 * xzDecRun.
 */
func xzDecInit(dictMax uint32, header *Header) *xzDec {
	s := new(xzDec)
	s.crc32 = crc32.NewIEEE()
	s.Header = header
	s.block.hash.sha256 = sha256.New()
	s.index.hash.sha256 = sha256.New()
	s.lzma2 = xzDecLZMA2Create(dictMax)
	xzDecReset(s)
	return s
}

/**
 * xzDecReset - Reset an already allocated decoder state
 * @s:          Decoder state allocated using xzDecInit
 *
 * This function can be used to reset the decoder state without
 * reallocating memory with xzDecInit.
 */
func xzDecReset(s *xzDec) {
	s.sequence = seqStreamHeader
	s.allowBufError = false
	s.pos = 0
	s.crc32.Reset()
	s.check = nil
	s.CheckType = checkUnset
	s.block.compressed = 0
	s.block.uncompressed = 0
	s.block.count = 0
	s.block.hash.unpadded = 0
	s.block.hash.uncompressed = 0
	s.block.hash.sha256.Reset()
	s.index.sequence = seqIndexCount
	s.index.size = 0
	s.index.count = 0
	s.index.hash.unpadded = 0
	s.index.hash.uncompressed = 0
	s.index.hash.sha256.Reset()
	s.temp.pos = 0
	s.temp.buf = s.temp.bufArray[:streamHeaderSize]
	s.chain = nil
	s.bcjsUsed = 0
	s.deltasUsed = 0
}
