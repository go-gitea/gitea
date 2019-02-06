package nodb

import (
	"encoding/binary"
	"errors"
	"sort"
	"time"

	"github.com/lunny/nodb/store"
)

const (
	OPand uint8 = iota + 1
	OPor
	OPxor
	OPnot
)

type BitPair struct {
	Pos int32
	Val uint8
}

type segBitInfo struct {
	Seq uint32
	Off uint32
	Val uint8
}

type segBitInfoArray []segBitInfo

const (
	// byte
	segByteWidth uint32 = 9
	segByteSize  uint32 = 1 << segByteWidth

	// bit
	segBitWidth uint32 = segByteWidth + 3
	segBitSize  uint32 = segByteSize << 3

	maxByteSize uint32 = 8 << 20
	maxSegCount uint32 = maxByteSize / segByteSize

	minSeq uint32 = 0
	maxSeq uint32 = uint32((maxByteSize << 3) - 1)
)

var bitsInByte = [256]int32{0, 1, 1, 2, 1, 2, 2, 3, 1, 2, 2, 3, 2, 3, 3,
	4, 1, 2, 2, 3, 2, 3, 3, 4, 2, 3, 3, 4, 3, 4, 4, 5, 1, 2, 2, 3, 2, 3,
	3, 4, 2, 3, 3, 4, 3, 4, 4, 5, 2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4,
	5, 5, 6, 1, 2, 2, 3, 2, 3, 3, 4, 2, 3, 3, 4, 3, 4, 4, 5, 2, 3, 3, 4,
	3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6, 2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4,
	5, 4, 5, 5, 6, 3, 4, 4, 5, 4, 5, 5, 6, 4, 5, 5, 6, 5, 6, 6, 7, 1, 2,
	2, 3, 2, 3, 3, 4, 2, 3, 3, 4, 3, 4, 4, 5, 2, 3, 3, 4, 3, 4, 4, 5, 3,
	4, 4, 5, 4, 5, 5, 6, 2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6,
	3, 4, 4, 5, 4, 5, 5, 6, 4, 5, 5, 6, 5, 6, 6, 7, 2, 3, 3, 4, 3, 4, 4,
	5, 3, 4, 4, 5, 4, 5, 5, 6, 3, 4, 4, 5, 4, 5, 5, 6, 4, 5, 5, 6, 5, 6,
	6, 7, 3, 4, 4, 5, 4, 5, 5, 6, 4, 5, 5, 6, 5, 6, 6, 7, 4, 5, 5, 6, 5,
	6, 6, 7, 5, 6, 6, 7, 6, 7, 7, 8}

var fillBits = [...]uint8{1, 3, 7, 15, 31, 63, 127, 255}

var emptySegment []byte = make([]byte, segByteSize, segByteSize)

var fillSegment []byte = func() []byte {
	data := make([]byte, segByteSize, segByteSize)
	for i := uint32(0); i < segByteSize; i++ {
		data[i] = 0xff
	}
	return data
}()

var errBinKey = errors.New("invalid bin key")
var errOffset = errors.New("invalid offset")
var errDuplicatePos = errors.New("duplicate bit pos")

func getBit(sz []byte, offset uint32) uint8 {
	index := offset >> 3
	if index >= uint32(len(sz)) {
		return 0 // error("overflow")
	}

	offset -= index << 3
	return sz[index] >> offset & 1
}

func setBit(sz []byte, offset uint32, val uint8) bool {
	if val != 1 && val != 0 {
		return false // error("invalid val")
	}

	index := offset >> 3
	if index >= uint32(len(sz)) {
		return false // error("overflow")
	}

	offset -= index << 3
	if sz[index]>>offset&1 != val {
		sz[index] ^= (1 << offset)
	}
	return true
}

func (datas segBitInfoArray) Len() int {
	return len(datas)
}

func (datas segBitInfoArray) Less(i, j int) bool {
	res := (datas)[i].Seq < (datas)[j].Seq
	if !res && (datas)[i].Seq == (datas)[j].Seq {
		res = (datas)[i].Off < (datas)[j].Off
	}
	return res
}

func (datas segBitInfoArray) Swap(i, j int) {
	datas[i], datas[j] = datas[j], datas[i]
}

func (db *DB) bEncodeMetaKey(key []byte) []byte {
	mk := make([]byte, len(key)+2)
	mk[0] = db.index
	mk[1] = BitMetaType

	copy(mk[2:], key)
	return mk
}

func (db *DB) bDecodeMetaKey(bkey []byte) ([]byte, error) {
	if len(bkey) < 2 || bkey[0] != db.index || bkey[1] != BitMetaType {
		return nil, errBinKey
	}

	return bkey[2:], nil
}

func (db *DB) bEncodeBinKey(key []byte, seq uint32) []byte {
	bk := make([]byte, len(key)+8)

	pos := 0
	bk[pos] = db.index
	pos++
	bk[pos] = BitType
	pos++

	binary.BigEndian.PutUint16(bk[pos:], uint16(len(key)))
	pos += 2

	copy(bk[pos:], key)
	pos += len(key)

	binary.BigEndian.PutUint32(bk[pos:], seq)

	return bk
}

func (db *DB) bDecodeBinKey(bkey []byte) (key []byte, seq uint32, err error) {
	if len(bkey) < 8 || bkey[0] != db.index {
		err = errBinKey
		return
	}

	keyLen := binary.BigEndian.Uint16(bkey[2:4])
	if int(keyLen+8) != len(bkey) {
		err = errBinKey
		return
	}

	key = bkey[4 : 4+keyLen]
	seq = uint32(binary.BigEndian.Uint32(bkey[4+keyLen:]))
	return
}

func (db *DB) bCapByteSize(seq uint32, off uint32) uint32 {
	var offByteSize uint32 = (off >> 3) + 1
	if offByteSize > segByteSize {
		offByteSize = segByteSize
	}

	return seq<<segByteWidth + offByteSize
}

func (db *DB) bParseOffset(key []byte, offset int32) (seq uint32, off uint32, err error) {
	if offset < 0 {
		if tailSeq, tailOff, e := db.bGetMeta(key); e != nil {
			err = e
			return
		} else if tailSeq >= 0 {
			offset += int32((uint32(tailSeq)<<segBitWidth | uint32(tailOff)) + 1)
			if offset < 0 {
				err = errOffset
				return
			}
		}
	}

	off = uint32(offset)

	seq = off >> segBitWidth
	off &= (segBitSize - 1)
	return
}

func (db *DB) bGetMeta(key []byte) (tailSeq int32, tailOff int32, err error) {
	var v []byte

	mk := db.bEncodeMetaKey(key)
	v, err = db.bucket.Get(mk)
	if err != nil {
		return
	}

	if v != nil {
		tailSeq = int32(binary.LittleEndian.Uint32(v[0:4]))
		tailOff = int32(binary.LittleEndian.Uint32(v[4:8]))
	} else {
		tailSeq = -1
		tailOff = -1
	}
	return
}

func (db *DB) bSetMeta(t *batch, key []byte, tailSeq uint32, tailOff uint32) {
	ek := db.bEncodeMetaKey(key)

	buf := make([]byte, 8)
	binary.LittleEndian.PutUint32(buf[0:4], tailSeq)
	binary.LittleEndian.PutUint32(buf[4:8], tailOff)

	t.Put(ek, buf)
	return
}

func (db *DB) bUpdateMeta(t *batch, key []byte, seq uint32, off uint32) (tailSeq uint32, tailOff uint32, err error) {
	var tseq, toff int32
	var update bool = false

	if tseq, toff, err = db.bGetMeta(key); err != nil {
		return
	} else if tseq < 0 {
		update = true
	} else {
		tailSeq = uint32(MaxInt32(tseq, 0))
		tailOff = uint32(MaxInt32(toff, 0))
		update = (seq > tailSeq || (seq == tailSeq && off > tailOff))
	}

	if update {
		db.bSetMeta(t, key, seq, off)
		tailSeq = seq
		tailOff = off
	}
	return
}

func (db *DB) bDelete(t *batch, key []byte) (drop int64) {
	mk := db.bEncodeMetaKey(key)
	t.Delete(mk)

	minKey := db.bEncodeBinKey(key, minSeq)
	maxKey := db.bEncodeBinKey(key, maxSeq)
	it := db.bucket.RangeIterator(minKey, maxKey, store.RangeClose)
	for ; it.Valid(); it.Next() {
		t.Delete(it.RawKey())
		drop++
	}
	it.Close()

	return drop
}

func (db *DB) bGetSegment(key []byte, seq uint32) ([]byte, []byte, error) {
	bk := db.bEncodeBinKey(key, seq)
	segment, err := db.bucket.Get(bk)
	if err != nil {
		return bk, nil, err
	}
	return bk, segment, nil
}

func (db *DB) bAllocateSegment(key []byte, seq uint32) ([]byte, []byte, error) {
	bk, segment, err := db.bGetSegment(key, seq)
	if err == nil && segment == nil {
		segment = make([]byte, segByteSize, segByteSize)
	}
	return bk, segment, err
}

func (db *DB) bIterator(key []byte) *store.RangeLimitIterator {
	sk := db.bEncodeBinKey(key, minSeq)
	ek := db.bEncodeBinKey(key, maxSeq)
	return db.bucket.RangeIterator(sk, ek, store.RangeClose)
}

func (db *DB) bSegAnd(a []byte, b []byte, res *[]byte) {
	if a == nil || b == nil {
		*res = nil
		return
	}

	data := *res
	if data == nil {
		data = make([]byte, segByteSize, segByteSize)
		*res = data
	}

	for i := uint32(0); i < segByteSize; i++ {
		data[i] = a[i] & b[i]
	}
	return
}

func (db *DB) bSegOr(a []byte, b []byte, res *[]byte) {
	if a == nil || b == nil {
		if a == nil && b == nil {
			*res = nil
		} else if a == nil {
			*res = b
		} else {
			*res = a
		}
		return
	}

	data := *res
	if data == nil {
		data = make([]byte, segByteSize, segByteSize)
		*res = data
	}

	for i := uint32(0); i < segByteSize; i++ {
		data[i] = a[i] | b[i]
	}
	return
}

func (db *DB) bSegXor(a []byte, b []byte, res *[]byte) {
	if a == nil && b == nil {
		*res = fillSegment
		return
	}

	if a == nil {
		a = emptySegment
	}

	if b == nil {
		b = emptySegment
	}

	data := *res
	if data == nil {
		data = make([]byte, segByteSize, segByteSize)
		*res = data
	}

	for i := uint32(0); i < segByteSize; i++ {
		data[i] = a[i] ^ b[i]
	}

	return
}

func (db *DB) bExpireAt(key []byte, when int64) (int64, error) {
	t := db.binBatch
	t.Lock()
	defer t.Unlock()

	if seq, _, err := db.bGetMeta(key); err != nil || seq < 0 {
		return 0, err
	} else {
		db.expireAt(t, BitType, key, when)
		if err := t.Commit(); err != nil {
			return 0, err
		}
	}
	return 1, nil
}

func (db *DB) bCountByte(val byte, soff uint32, eoff uint32) int32 {
	if soff > eoff {
		soff, eoff = eoff, soff
	}

	mask := uint8(0)
	if soff > 0 {
		mask |= fillBits[soff-1]
	}
	if eoff < 7 {
		mask |= (fillBits[7] ^ fillBits[eoff])
	}
	mask = fillBits[7] ^ mask

	return bitsInByte[val&mask]
}

func (db *DB) bCountSeg(key []byte, seq uint32, soff uint32, eoff uint32) (cnt int32, err error) {
	if soff >= segBitSize || soff < 0 ||
		eoff >= segBitSize || eoff < 0 {
		return
	}

	var segment []byte
	if _, segment, err = db.bGetSegment(key, seq); err != nil {
		return
	}

	if segment == nil {
		return
	}

	if soff > eoff {
		soff, eoff = eoff, soff
	}

	headIdx := int(soff >> 3)
	endIdx := int(eoff >> 3)
	sByteOff := soff - ((soff >> 3) << 3)
	eByteOff := eoff - ((eoff >> 3) << 3)

	if headIdx == endIdx {
		cnt = db.bCountByte(segment[headIdx], sByteOff, eByteOff)
	} else {
		cnt = db.bCountByte(segment[headIdx], sByteOff, 7) +
			db.bCountByte(segment[endIdx], 0, eByteOff)
	}

	// sum up following bytes
	for idx, end := headIdx+1, endIdx-1; idx <= end; idx += 1 {
		cnt += bitsInByte[segment[idx]]
		if idx == end {
			break
		}
	}

	return
}

func (db *DB) BGet(key []byte) (data []byte, err error) {
	if err = checkKeySize(key); err != nil {
		return
	}

	var ts, to int32
	if ts, to, err = db.bGetMeta(key); err != nil || ts < 0 {
		return
	}

	var tailSeq, tailOff = uint32(ts), uint32(to)
	var capByteSize uint32 = db.bCapByteSize(tailSeq, tailOff)
	data = make([]byte, capByteSize, capByteSize)

	minKey := db.bEncodeBinKey(key, minSeq)
	maxKey := db.bEncodeBinKey(key, tailSeq)
	it := db.bucket.RangeIterator(minKey, maxKey, store.RangeClose)

	var seq, s, e uint32
	for ; it.Valid(); it.Next() {
		if _, seq, err = db.bDecodeBinKey(it.RawKey()); err != nil {
			data = nil
			break
		}

		s = seq << segByteWidth
		e = MinUInt32(s+segByteSize, capByteSize)
		copy(data[s:e], it.RawValue())
	}
	it.Close()

	return
}

func (db *DB) BDelete(key []byte) (drop int64, err error) {
	if err = checkKeySize(key); err != nil {
		return
	}

	t := db.binBatch
	t.Lock()
	defer t.Unlock()

	drop = db.bDelete(t, key)
	db.rmExpire(t, BitType, key)

	err = t.Commit()
	return
}

func (db *DB) BSetBit(key []byte, offset int32, val uint8) (ori uint8, err error) {
	if err = checkKeySize(key); err != nil {
		return
	}

	//	todo : check offset
	var seq, off uint32
	if seq, off, err = db.bParseOffset(key, offset); err != nil {
		return 0, err
	}

	var bk, segment []byte
	if bk, segment, err = db.bAllocateSegment(key, seq); err != nil {
		return 0, err
	}

	if segment != nil {
		ori = getBit(segment, off)
		if setBit(segment, off, val) {
			t := db.binBatch
			t.Lock()
			defer t.Unlock()

			t.Put(bk, segment)
			if _, _, e := db.bUpdateMeta(t, key, seq, off); e != nil {
				err = e
				return
			}

			err = t.Commit()
		}
	}

	return
}

func (db *DB) BMSetBit(key []byte, args ...BitPair) (place int64, err error) {
	if err = checkKeySize(key); err != nil {
		return
	}

	//	(ps : so as to aviod wasting memory copy while calling db.Get() and batch.Put(),
	//		  here we sequence the params by pos, so that we can merge the execution of
	//		  diff pos setting which targets on the same segment respectively. )

	//	#1 : sequence request data
	var argCnt = len(args)
	var bitInfos segBitInfoArray = make(segBitInfoArray, argCnt)
	var seq, off uint32

	for i, info := range args {
		if seq, off, err = db.bParseOffset(key, info.Pos); err != nil {
			return
		}

		bitInfos[i].Seq = seq
		bitInfos[i].Off = off
		bitInfos[i].Val = info.Val
	}

	sort.Sort(bitInfos)

	for i := 1; i < argCnt; i++ {
		if bitInfos[i].Seq == bitInfos[i-1].Seq && bitInfos[i].Off == bitInfos[i-1].Off {
			return 0, errDuplicatePos
		}
	}

	//	#2 : execute bit set in order
	t := db.binBatch
	t.Lock()
	defer t.Unlock()

	var curBinKey, curSeg []byte
	var curSeq, maxSeq, maxOff uint32

	for _, info := range bitInfos {
		if curSeg != nil && info.Seq != curSeq {
			t.Put(curBinKey, curSeg)
			curSeg = nil
		}

		if curSeg == nil {
			curSeq = info.Seq
			if curBinKey, curSeg, err = db.bAllocateSegment(key, info.Seq); err != nil {
				return
			}

			if curSeg == nil {
				continue
			}
		}

		if setBit(curSeg, info.Off, info.Val) {
			maxSeq = info.Seq
			maxOff = info.Off
			place++
		}
	}

	if curSeg != nil {
		t.Put(curBinKey, curSeg)
	}

	//	finally, update meta
	if place > 0 {
		if _, _, err = db.bUpdateMeta(t, key, maxSeq, maxOff); err != nil {
			return
		}

		err = t.Commit()
	}

	return
}

func (db *DB) BGetBit(key []byte, offset int32) (uint8, error) {
	if seq, off, err := db.bParseOffset(key, offset); err != nil {
		return 0, err
	} else {
		_, segment, err := db.bGetSegment(key, seq)
		if err != nil {
			return 0, err
		}

		if segment == nil {
			return 0, nil
		} else {
			return getBit(segment, off), nil
		}
	}
}

// func (db *DB) BGetRange(key []byte, start int32, end int32) ([]byte, error) {
// 	section := make([]byte)

// 	return
// }

func (db *DB) BCount(key []byte, start int32, end int32) (cnt int32, err error) {
	var sseq, soff uint32
	if sseq, soff, err = db.bParseOffset(key, start); err != nil {
		return
	}

	var eseq, eoff uint32
	if eseq, eoff, err = db.bParseOffset(key, end); err != nil {
		return
	}

	if sseq > eseq || (sseq == eseq && soff > eoff) {
		sseq, eseq = eseq, sseq
		soff, eoff = eoff, soff
	}

	var segCnt int32
	if eseq == sseq {
		if segCnt, err = db.bCountSeg(key, sseq, soff, eoff); err != nil {
			return 0, err
		}

		cnt = segCnt

	} else {
		if segCnt, err = db.bCountSeg(key, sseq, soff, segBitSize-1); err != nil {
			return 0, err
		} else {
			cnt += segCnt
		}

		if segCnt, err = db.bCountSeg(key, eseq, 0, eoff); err != nil {
			return 0, err
		} else {
			cnt += segCnt
		}
	}

	//	middle segs
	var segment []byte
	skey := db.bEncodeBinKey(key, sseq)
	ekey := db.bEncodeBinKey(key, eseq)

	it := db.bucket.RangeIterator(skey, ekey, store.RangeOpen)
	for ; it.Valid(); it.Next() {
		segment = it.RawValue()
		for _, bt := range segment {
			cnt += bitsInByte[bt]
		}
	}
	it.Close()

	return
}

func (db *DB) BTail(key []byte) (int32, error) {
	// effective length of data, the highest bit-pos set in history
	tailSeq, tailOff, err := db.bGetMeta(key)
	if err != nil {
		return 0, err
	}

	tail := int32(-1)
	if tailSeq >= 0 {
		tail = int32(uint32(tailSeq)<<segBitWidth | uint32(tailOff))
	}

	return tail, nil
}

func (db *DB) BOperation(op uint8, dstkey []byte, srckeys ...[]byte) (blen int32, err error) {
	//	blen -
	//		the total bit size of data stored in destination key,
	//		that is equal to the size of the longest input string.

	var exeOp func([]byte, []byte, *[]byte)
	switch op {
	case OPand:
		exeOp = db.bSegAnd
	case OPor:
		exeOp = db.bSegOr
	case OPxor, OPnot:
		exeOp = db.bSegXor
	default:
		return
	}

	if dstkey == nil || srckeys == nil {
		return
	}

	t := db.binBatch
	t.Lock()
	defer t.Unlock()

	var srcKseq, srcKoff int32
	var seq, off, maxDstSeq, maxDstOff uint32

	var keyNum int = len(srckeys)
	var validKeyNum int
	for i := 0; i < keyNum; i++ {
		if srcKseq, srcKoff, err = db.bGetMeta(srckeys[i]); err != nil {
			return
		} else if srcKseq < 0 {
			srckeys[i] = nil
			continue
		}

		validKeyNum++

		seq = uint32(srcKseq)
		off = uint32(srcKoff)
		if seq > maxDstSeq || (seq == maxDstSeq && off > maxDstOff) {
			maxDstSeq = seq
			maxDstOff = off
		}
	}

	if (op == OPnot && validKeyNum != 1) ||
		(op != OPnot && validKeyNum < 2) {
		return // with not enough existing source key
	}

	var srcIdx int
	for srcIdx = 0; srcIdx < keyNum; srcIdx++ {
		if srckeys[srcIdx] != nil {
			break
		}
	}

	// init - data
	var segments = make([][]byte, maxDstSeq+1)

	if op == OPnot {
		//	ps :
		//		( ~num == num ^ 0x11111111 )
		//		we init the result segments with all bit set,
		//		then we can calculate through the way of 'xor'.

		//	ahead segments bin format : 1111 ... 1111
		for i := uint32(0); i < maxDstSeq; i++ {
			segments[i] = fillSegment
		}

		//	last segment bin format : 1111..1100..0000
		var tailSeg = make([]byte, segByteSize, segByteSize)
		var fillByte = fillBits[7]
		var tailSegLen = db.bCapByteSize(uint32(0), maxDstOff)
		for i := uint32(0); i < tailSegLen-1; i++ {
			tailSeg[i] = fillByte
		}
		tailSeg[tailSegLen-1] = fillBits[maxDstOff-(tailSegLen-1)<<3]
		segments[maxDstSeq] = tailSeg

	} else {
		// ps : init segments by data corresponding to the 1st valid source key
		it := db.bIterator(srckeys[srcIdx])
		for ; it.Valid(); it.Next() {
			if _, seq, err = db.bDecodeBinKey(it.RawKey()); err != nil {
				// to do ...
				it.Close()
				return
			}
			segments[seq] = it.Value()
		}
		it.Close()
		srcIdx++
	}

	//	operation with following keys
	var res []byte
	for i := srcIdx; i < keyNum; i++ {
		if srckeys[i] == nil {
			continue
		}

		it := db.bIterator(srckeys[i])
		for idx, end := uint32(0), false; !end; it.Next() {
			end = !it.Valid()
			if !end {
				if _, seq, err = db.bDecodeBinKey(it.RawKey()); err != nil {
					// to do ...
					it.Close()
					return
				}
			} else {
				seq = maxDstSeq + 1
			}

			// todo :
			// 		operation 'and' can be optimize here :
			//		if seq > max_segments_idx, this loop can be break,
			//		which can avoid cost from Key() and bDecodeBinKey()

			for ; idx < seq; idx++ {
				res = nil
				exeOp(segments[idx], nil, &res)
				segments[idx] = res
			}

			if !end {
				res = it.Value()
				exeOp(segments[seq], res, &res)
				segments[seq] = res
				idx++
			}
		}
		it.Close()
	}

	// clear the old data in case
	db.bDelete(t, dstkey)
	db.rmExpire(t, BitType, dstkey)

	//	set data
	db.bSetMeta(t, dstkey, maxDstSeq, maxDstOff)

	var bk []byte
	for seq, segt := range segments {
		if segt != nil {
			bk = db.bEncodeBinKey(dstkey, uint32(seq))
			t.Put(bk, segt)
		}
	}

	err = t.Commit()
	if err == nil {
		// blen = int32(db.bCapByteSize(maxDstOff, maxDstOff))
		blen = int32(maxDstSeq<<segBitWidth | maxDstOff + 1)
	}

	return
}

func (db *DB) BExpire(key []byte, duration int64) (int64, error) {
	if duration <= 0 {
		return 0, errExpireValue
	}

	if err := checkKeySize(key); err != nil {
		return -1, err
	}

	return db.bExpireAt(key, time.Now().Unix()+duration)
}

func (db *DB) BExpireAt(key []byte, when int64) (int64, error) {
	if when <= time.Now().Unix() {
		return 0, errExpireValue
	}

	if err := checkKeySize(key); err != nil {
		return -1, err
	}

	return db.bExpireAt(key, when)
}

func (db *DB) BTTL(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return -1, err
	}

	return db.ttl(BitType, key)
}

func (db *DB) BPersist(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	t := db.binBatch
	t.Lock()
	defer t.Unlock()

	n, err := db.rmExpire(t, BitType, key)
	if err != nil {
		return 0, err
	}

	err = t.Commit()
	return n, err
}

func (db *DB) BScan(key []byte, count int, inclusive bool, match string) ([][]byte, error) {
	return db.scan(BitMetaType, key, count, inclusive, match)
}

func (db *DB) bFlush() (drop int64, err error) {
	t := db.binBatch
	t.Lock()
	defer t.Unlock()

	return db.flushType(t, BitType)
}
