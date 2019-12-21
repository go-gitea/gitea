// Copyright (c) 2012-2018 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

import (
	"math"
	"time"
)

const (
	_               uint8 = iota
	simpleVdNil           = 1
	simpleVdFalse         = 2
	simpleVdTrue          = 3
	simpleVdFloat32       = 4
	simpleVdFloat64       = 5

	// each lasts for 4 (ie n, n+1, n+2, n+3)
	simpleVdPosInt = 8
	simpleVdNegInt = 12

	simpleVdTime = 24

	// containers: each lasts for 4 (ie n, n+1, n+2, ... n+7)
	simpleVdString    = 216
	simpleVdByteArray = 224
	simpleVdArray     = 232
	simpleVdMap       = 240
	simpleVdExt       = 248
)

type simpleEncDriver struct {
	noBuiltInTypes
	encDriverNoopContainerWriter
	h *SimpleHandle
	b [8]byte
	_ [6]uint64 // padding (cache-aligned)
	e Encoder
}

func (e *simpleEncDriver) encoder() *Encoder {
	return &e.e
}

func (e *simpleEncDriver) EncodeNil() {
	e.e.encWr.writen1(simpleVdNil)
}

func (e *simpleEncDriver) EncodeBool(b bool) {
	if e.h.EncZeroValuesAsNil && e.e.c != containerMapKey && !b {
		e.EncodeNil()
		return
	}
	if b {
		e.e.encWr.writen1(simpleVdTrue)
	} else {
		e.e.encWr.writen1(simpleVdFalse)
	}
}

func (e *simpleEncDriver) EncodeFloat32(f float32) {
	if e.h.EncZeroValuesAsNil && e.e.c != containerMapKey && f == 0.0 {
		e.EncodeNil()
		return
	}
	e.e.encWr.writen1(simpleVdFloat32)
	bigenHelper{e.b[:4], e.e.w()}.writeUint32(math.Float32bits(f))
}

func (e *simpleEncDriver) EncodeFloat64(f float64) {
	if e.h.EncZeroValuesAsNil && e.e.c != containerMapKey && f == 0.0 {
		e.EncodeNil()
		return
	}
	e.e.encWr.writen1(simpleVdFloat64)
	bigenHelper{e.b[:8], e.e.w()}.writeUint64(math.Float64bits(f))
}

func (e *simpleEncDriver) EncodeInt(v int64) {
	if v < 0 {
		e.encUint(uint64(-v), simpleVdNegInt)
	} else {
		e.encUint(uint64(v), simpleVdPosInt)
	}
}

func (e *simpleEncDriver) EncodeUint(v uint64) {
	e.encUint(v, simpleVdPosInt)
}

func (e *simpleEncDriver) encUint(v uint64, bd uint8) {
	if e.h.EncZeroValuesAsNil && e.e.c != containerMapKey && v == 0 {
		e.EncodeNil()
		return
	}
	if v <= math.MaxUint8 {
		e.e.encWr.writen2(bd, uint8(v))
	} else if v <= math.MaxUint16 {
		e.e.encWr.writen1(bd + 1)
		bigenHelper{e.b[:2], e.e.w()}.writeUint16(uint16(v))
	} else if v <= math.MaxUint32 {
		e.e.encWr.writen1(bd + 2)
		bigenHelper{e.b[:4], e.e.w()}.writeUint32(uint32(v))
	} else { // if v <= math.MaxUint64 {
		e.e.encWr.writen1(bd + 3)
		bigenHelper{e.b[:8], e.e.w()}.writeUint64(v)
	}
}

func (e *simpleEncDriver) encLen(bd byte, length int) {
	if length == 0 {
		e.e.encWr.writen1(bd)
	} else if length <= math.MaxUint8 {
		e.e.encWr.writen1(bd + 1)
		e.e.encWr.writen1(uint8(length))
	} else if length <= math.MaxUint16 {
		e.e.encWr.writen1(bd + 2)
		bigenHelper{e.b[:2], e.e.w()}.writeUint16(uint16(length))
	} else if int64(length) <= math.MaxUint32 {
		e.e.encWr.writen1(bd + 3)
		bigenHelper{e.b[:4], e.e.w()}.writeUint32(uint32(length))
	} else {
		e.e.encWr.writen1(bd + 4)
		bigenHelper{e.b[:8], e.e.w()}.writeUint64(uint64(length))
	}
}

func (e *simpleEncDriver) EncodeExt(v interface{}, xtag uint64, ext Ext) {
	var bs []byte
	if ext == SelfExt {
		bs = e.e.blist.get(1024)[:0]
		e.e.sideEncode(v, &bs)
	} else {
		bs = ext.WriteExt(v)
	}
	if bs == nil {
		e.EncodeNil()
		return
	}
	e.encodeExtPreamble(uint8(xtag), len(bs))
	e.e.encWr.writeb(bs)
	if ext == SelfExt {
		e.e.blist.put(bs)
	}
}

func (e *simpleEncDriver) EncodeRawExt(re *RawExt) {
	e.encodeExtPreamble(uint8(re.Tag), len(re.Data))
	e.e.encWr.writeb(re.Data)
}

func (e *simpleEncDriver) encodeExtPreamble(xtag byte, length int) {
	e.encLen(simpleVdExt, length)
	e.e.encWr.writen1(xtag)
}

func (e *simpleEncDriver) WriteArrayStart(length int) {
	e.encLen(simpleVdArray, length)
}

func (e *simpleEncDriver) WriteMapStart(length int) {
	e.encLen(simpleVdMap, length)
}

func (e *simpleEncDriver) EncodeString(v string) {
	if e.h.EncZeroValuesAsNil && e.e.c != containerMapKey && v == "" {
		e.EncodeNil()
		return
	}
	if e.h.StringToRaw {
		e.encLen(simpleVdByteArray, len(v))
	} else {
		e.encLen(simpleVdString, len(v))
	}
	e.e.encWr.writestr(v)
}

func (e *simpleEncDriver) EncodeStringBytesRaw(v []byte) {
	// if e.h.EncZeroValuesAsNil && e.c != containerMapKey && v == nil {
	if v == nil {
		e.EncodeNil()
		return
	}
	e.encLen(simpleVdByteArray, len(v))
	e.e.encWr.writeb(v)
}

func (e *simpleEncDriver) EncodeTime(t time.Time) {
	// if e.h.EncZeroValuesAsNil && e.c != containerMapKey && t.IsZero() {
	if t.IsZero() {
		e.EncodeNil()
		return
	}
	v, err := t.MarshalBinary()
	if err != nil {
		e.e.errorv(err)
		return
	}
	// time.Time marshalbinary takes about 14 bytes.
	e.e.encWr.writen2(simpleVdTime, uint8(len(v)))
	e.e.encWr.writeb(v)
}

//------------------------------------

type simpleDecDriver struct {
	h      *SimpleHandle
	bdRead bool
	bd     byte
	fnil   bool
	noBuiltInTypes
	decDriverNoopContainerReader
	_ [6]uint64 // padding
	d Decoder
}

func (d *simpleDecDriver) decoder() *Decoder {
	return &d.d
}

func (d *simpleDecDriver) readNextBd() {
	d.bd = d.d.decRd.readn1()
	d.bdRead = true
}

func (d *simpleDecDriver) uncacheRead() {
	if d.bdRead {
		d.d.decRd.unreadn1()
		d.bdRead = false
	}
}

func (d *simpleDecDriver) advanceNil() (null bool) {
	d.fnil = false
	if !d.bdRead {
		d.readNextBd()
	}
	if d.bd == simpleVdNil {
		d.bdRead = false
		d.fnil = true
		null = true
	}
	return
}

func (d *simpleDecDriver) Nil() bool {
	return d.fnil
}

func (d *simpleDecDriver) ContainerType() (vt valueType) {
	if !d.bdRead {
		d.readNextBd()
	}
	d.fnil = false
	switch d.bd {
	case simpleVdNil:
		d.bdRead = false
		d.fnil = true
		return valueTypeNil
	case simpleVdByteArray, simpleVdByteArray + 1,
		simpleVdByteArray + 2, simpleVdByteArray + 3, simpleVdByteArray + 4:
		return valueTypeBytes
	case simpleVdString, simpleVdString + 1,
		simpleVdString + 2, simpleVdString + 3, simpleVdString + 4:
		return valueTypeString
	case simpleVdArray, simpleVdArray + 1,
		simpleVdArray + 2, simpleVdArray + 3, simpleVdArray + 4:
		return valueTypeArray
	case simpleVdMap, simpleVdMap + 1,
		simpleVdMap + 2, simpleVdMap + 3, simpleVdMap + 4:
		return valueTypeMap
	}
	return valueTypeUnset
}

func (d *simpleDecDriver) TryNil() bool {
	return d.advanceNil()
}

func (d *simpleDecDriver) decCheckInteger() (ui uint64, neg bool) {
	switch d.bd {
	case simpleVdPosInt:
		ui = uint64(d.d.decRd.readn1())
	case simpleVdPosInt + 1:
		ui = uint64(bigen.Uint16(d.d.decRd.readx(2)))
	case simpleVdPosInt + 2:
		ui = uint64(bigen.Uint32(d.d.decRd.readx(4)))
	case simpleVdPosInt + 3:
		ui = uint64(bigen.Uint64(d.d.decRd.readx(8)))
	case simpleVdNegInt:
		ui = uint64(d.d.decRd.readn1())
		neg = true
	case simpleVdNegInt + 1:
		ui = uint64(bigen.Uint16(d.d.decRd.readx(2)))
		neg = true
	case simpleVdNegInt + 2:
		ui = uint64(bigen.Uint32(d.d.decRd.readx(4)))
		neg = true
	case simpleVdNegInt + 3:
		ui = uint64(bigen.Uint64(d.d.decRd.readx(8)))
		neg = true
	default:
		d.d.errorf("integer only valid from pos/neg integer1..8. Invalid descriptor: %v", d.bd)
		return
	}
	// DO NOT do this check below, because callers may only want the unsigned value:
	//
	// if ui > math.MaxInt64 {
	// 	d.d.errorf("decIntAny: Integer out of range for signed int64: %v", ui)
	//		return
	// }
	return
}

func (d *simpleDecDriver) DecodeInt64() (i int64) {
	if d.advanceNil() {
		return
	}
	ui, neg := d.decCheckInteger()
	i = chkOvf.SignedIntV(ui)
	if neg {
		i = -i
	}
	d.bdRead = false
	return
}

func (d *simpleDecDriver) DecodeUint64() (ui uint64) {
	if d.advanceNil() {
		return
	}
	ui, neg := d.decCheckInteger()
	if neg {
		d.d.errorf("assigning negative signed value to unsigned type")
		return
	}
	d.bdRead = false
	return
}

func (d *simpleDecDriver) DecodeFloat64() (f float64) {
	if d.advanceNil() {
		return
	}
	if d.bd == simpleVdFloat32 {
		f = float64(math.Float32frombits(bigen.Uint32(d.d.decRd.readx(4))))
	} else if d.bd == simpleVdFloat64 {
		f = math.Float64frombits(bigen.Uint64(d.d.decRd.readx(8)))
	} else {
		if d.bd >= simpleVdPosInt && d.bd <= simpleVdNegInt+3 {
			f = float64(d.DecodeInt64())
		} else {
			d.d.errorf("float only valid from float32/64: Invalid descriptor: %v", d.bd)
			return
		}
	}
	d.bdRead = false
	return
}

// bool can be decoded from bool only (single byte).
func (d *simpleDecDriver) DecodeBool() (b bool) {
	if d.advanceNil() {
		return
	}
	if d.bd == simpleVdFalse {
	} else if d.bd == simpleVdTrue {
		b = true
	} else {
		d.d.errorf("cannot decode bool - %s: %x", msgBadDesc, d.bd)
		return
	}
	d.bdRead = false
	return
}

func (d *simpleDecDriver) ReadMapStart() (length int) {
	if d.advanceNil() {
		return decContainerLenNil
	}
	d.bdRead = false
	return d.decLen()
}

func (d *simpleDecDriver) ReadArrayStart() (length int) {
	if d.advanceNil() {
		return decContainerLenNil
	}
	d.bdRead = false
	return d.decLen()
}

func (d *simpleDecDriver) decLen() int {
	switch d.bd % 8 {
	case 0:
		return 0
	case 1:
		return int(d.d.decRd.readn1())
	case 2:
		return int(bigen.Uint16(d.d.decRd.readx(2)))
	case 3:
		ui := uint64(bigen.Uint32(d.d.decRd.readx(4)))
		if chkOvf.Uint(ui, intBitsize) {
			d.d.errorf("overflow integer: %v", ui)
			return 0
		}
		return int(ui)
	case 4:
		ui := bigen.Uint64(d.d.decRd.readx(8))
		if chkOvf.Uint(ui, intBitsize) {
			d.d.errorf("overflow integer: %v", ui)
			return 0
		}
		return int(ui)
	}
	d.d.errorf("cannot read length: bd%%8 must be in range 0..4. Got: %d", d.bd%8)
	return -1
}

func (d *simpleDecDriver) DecodeStringAsBytes() (s []byte) {
	return d.DecodeBytes(d.d.b[:], true)
}

func (d *simpleDecDriver) DecodeBytes(bs []byte, zerocopy bool) (bsOut []byte) {
	if d.advanceNil() {
		return
	}
	// check if an "array" of uint8's (see ContainerType for how to infer if an array)
	if d.bd >= simpleVdArray && d.bd <= simpleVdMap+4 {
		if len(bs) == 0 && zerocopy {
			bs = d.d.b[:]
		}
		// bsOut, _ = fastpathTV.DecSliceUint8V(bs, true, d.d)
		slen := d.ReadArrayStart()
		bs = usableByteSlice(bs, slen)
		for i := 0; i < len(bs); i++ {
			bs[i] = uint8(chkOvf.UintV(d.DecodeUint64(), 8))
		}
		return bs
	}

	clen := d.decLen()
	d.bdRead = false
	if zerocopy {
		if d.d.bytes {
			return d.d.decRd.readx(uint(clen))
		} else if len(bs) == 0 {
			bs = d.d.b[:]
		}
	}
	return decByteSlice(d.d.r(), clen, d.d.h.MaxInitLen, bs)
}

func (d *simpleDecDriver) DecodeTime() (t time.Time) {
	if d.advanceNil() {
		return
	}
	if d.bd != simpleVdTime {
		d.d.errorf("invalid descriptor for time.Time - expect 0x%x, received 0x%x", simpleVdTime, d.bd)
		return
	}
	d.bdRead = false
	clen := int(d.d.decRd.readn1())
	b := d.d.decRd.readx(uint(clen))
	if err := (&t).UnmarshalBinary(b); err != nil {
		d.d.errorv(err)
	}
	return
}

func (d *simpleDecDriver) DecodeExt(rv interface{}, xtag uint64, ext Ext) {
	if xtag > 0xff {
		d.d.errorf("ext: tag must be <= 0xff; got: %v", xtag)
		return
	}
	if d.advanceNil() {
		return
	}
	realxtag1, xbs := d.decodeExtV(ext != nil, uint8(xtag))
	realxtag := uint64(realxtag1)
	if ext == nil {
		re := rv.(*RawExt)
		re.Tag = realxtag
		re.Data = detachZeroCopyBytes(d.d.bytes, re.Data, xbs)
	} else if ext == SelfExt {
		d.d.sideDecode(rv, xbs)
	} else {
		ext.ReadExt(rv, xbs)
	}
}

func (d *simpleDecDriver) decodeExtV(verifyTag bool, tag byte) (xtag byte, xbs []byte) {
	switch d.bd {
	case simpleVdExt, simpleVdExt + 1, simpleVdExt + 2, simpleVdExt + 3, simpleVdExt + 4:
		l := d.decLen()
		xtag = d.d.decRd.readn1()
		if verifyTag && xtag != tag {
			d.d.errorf("wrong extension tag. Got %b. Expecting: %v", xtag, tag)
			return
		}
		if d.d.bytes {
			xbs = d.d.decRd.readx(uint(l))
		} else {
			xbs = decByteSlice(d.d.r(), l, d.d.h.MaxInitLen, d.d.b[:])
		}
	case simpleVdByteArray, simpleVdByteArray + 1,
		simpleVdByteArray + 2, simpleVdByteArray + 3, simpleVdByteArray + 4:
		xbs = d.DecodeBytes(nil, true)
	default:
		d.d.errorf("ext - %s - expecting extensions/bytearray, got: 0x%x", msgBadDesc, d.bd)
		return
	}
	d.bdRead = false
	return
}

func (d *simpleDecDriver) DecodeNaked() {
	if !d.bdRead {
		d.readNextBd()
	}

	d.fnil = false
	n := d.d.naked()
	var decodeFurther bool

	switch d.bd {
	case simpleVdNil:
		n.v = valueTypeNil
		d.fnil = true
	case simpleVdFalse:
		n.v = valueTypeBool
		n.b = false
	case simpleVdTrue:
		n.v = valueTypeBool
		n.b = true
	case simpleVdPosInt, simpleVdPosInt + 1, simpleVdPosInt + 2, simpleVdPosInt + 3:
		if d.h.SignedInteger {
			n.v = valueTypeInt
			n.i = d.DecodeInt64()
		} else {
			n.v = valueTypeUint
			n.u = d.DecodeUint64()
		}
	case simpleVdNegInt, simpleVdNegInt + 1, simpleVdNegInt + 2, simpleVdNegInt + 3:
		n.v = valueTypeInt
		n.i = d.DecodeInt64()
	case simpleVdFloat32:
		n.v = valueTypeFloat
		n.f = d.DecodeFloat64()
	case simpleVdFloat64:
		n.v = valueTypeFloat
		n.f = d.DecodeFloat64()
	case simpleVdTime:
		n.v = valueTypeTime
		n.t = d.DecodeTime()
	case simpleVdString, simpleVdString + 1,
		simpleVdString + 2, simpleVdString + 3, simpleVdString + 4:
		n.v = valueTypeString
		n.s = string(d.DecodeStringAsBytes())
	case simpleVdByteArray, simpleVdByteArray + 1,
		simpleVdByteArray + 2, simpleVdByteArray + 3, simpleVdByteArray + 4:
		decNakedReadRawBytes(d, &d.d, n, d.h.RawToString)
	case simpleVdExt, simpleVdExt + 1, simpleVdExt + 2, simpleVdExt + 3, simpleVdExt + 4:
		n.v = valueTypeExt
		l := d.decLen()
		n.u = uint64(d.d.decRd.readn1())
		if d.d.bytes {
			n.l = d.d.decRd.readx(uint(l))
		} else {
			n.l = decByteSlice(d.d.r(), l, d.d.h.MaxInitLen, d.d.b[:])
		}
	case simpleVdArray, simpleVdArray + 1, simpleVdArray + 2,
		simpleVdArray + 3, simpleVdArray + 4:
		n.v = valueTypeArray
		decodeFurther = true
	case simpleVdMap, simpleVdMap + 1, simpleVdMap + 2, simpleVdMap + 3, simpleVdMap + 4:
		n.v = valueTypeMap
		decodeFurther = true
	default:
		d.d.errorf("cannot infer value - %s 0x%x", msgBadDesc, d.bd)
	}

	if !decodeFurther {
		d.bdRead = false
	}
}

//------------------------------------

// SimpleHandle is a Handle for a very simple encoding format.
//
// simple is a simplistic codec similar to binc, but not as compact.
//   - Encoding of a value is always preceded by the descriptor byte (bd)
//   - True, false, nil are encoded fully in 1 byte (the descriptor)
//   - Integers (intXXX, uintXXX) are encoded in 1, 2, 4 or 8 bytes (plus a descriptor byte).
//     There are positive (uintXXX and intXXX >= 0) and negative (intXXX < 0) integers.
//   - Floats are encoded in 4 or 8 bytes (plus a descriptor byte)
//   - Length of containers (strings, bytes, array, map, extensions)
//     are encoded in 0, 1, 2, 4 or 8 bytes.
//     Zero-length containers have no length encoded.
//     For others, the number of bytes is given by pow(2, bd%3)
//   - maps are encoded as [bd] [length] [[key][value]]...
//   - arrays are encoded as [bd] [length] [value]...
//   - extensions are encoded as [bd] [length] [tag] [byte]...
//   - strings/bytearrays are encoded as [bd] [length] [byte]...
//   - time.Time are encoded as [bd] [length] [byte]...
//
// The full spec will be published soon.
type SimpleHandle struct {
	binaryEncodingType
	BasicHandle
	// EncZeroValuesAsNil says to encode zero values for numbers, bool, string, etc as nil
	EncZeroValuesAsNil bool

	_ [7]uint64 // padding (cache-aligned)
}

// Name returns the name of the handle: simple
func (h *SimpleHandle) Name() string { return "simple" }

func (h *SimpleHandle) newEncDriver() encDriver {
	var e = &simpleEncDriver{h: h}
	e.e.e = e
	e.e.init(h)
	e.reset()
	return e
}

func (h *SimpleHandle) newDecDriver() decDriver {
	d := &simpleDecDriver{h: h}
	d.d.d = d
	d.d.init(h)
	d.reset()
	return d
}

func (e *simpleEncDriver) reset() {
}

func (d *simpleDecDriver) reset() {
	d.bd, d.bdRead = 0, false
	d.fnil = false
}

var _ decDriver = (*simpleDecDriver)(nil)
var _ encDriver = (*simpleEncDriver)(nil)
