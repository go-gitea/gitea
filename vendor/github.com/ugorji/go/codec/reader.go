// Copyright (c) 2012-2018 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

import "io"

// decReader abstracts the reading source, allowing implementations that can
// read from an io.Reader or directly off a byte slice with zero-copying.
type decReader interface {
	unreadn1()
	// readx will use the implementation scratch buffer if possible i.e. n < len(scratchbuf), OR
	// just return a view of the []byte being decoded from.
	// Ensure you call detachZeroCopyBytes later if this needs to be sent outside codec control.
	readx(n uint) []byte
	readb([]byte)
	readn1() uint8
	// read up to 7 bytes at a time
	readn(num uint8) (v [rwNLen]byte)
	numread() uint // number of bytes read
	track()
	stopTrack() []byte

	// skip will skip any byte that matches, and return the first non-matching byte
	skip(accept *bitset256) (token byte)
	// readTo will read any byte that matches, stopping once no-longer matching.
	readTo(accept *bitset256) (out []byte)
	// readUntil will read, only stopping once it matches the 'stop' byte.
	readUntil(stop byte, includeLast bool) (out []byte)
}

// ------------------------------------------------

type unreadByteStatus uint8

// unreadByteStatus goes from
// undefined (when initialized) -- (read) --> canUnread -- (unread) --> canRead ...
const (
	unreadByteUndefined unreadByteStatus = iota
	unreadByteCanRead
	unreadByteCanUnread
)

// --------------------

type ioDecReaderCommon struct {
	r io.Reader // the reader passed in

	n uint // num read

	l   byte             // last byte
	ls  unreadByteStatus // last byte status
	trb bool             // tracking bytes turned on
	_   bool
	b   [4]byte // tiny buffer for reading single bytes

	blist *bytesFreelist

	tr   []byte // buffer for tracking bytes
	bufr []byte // buffer for readTo/readUntil
}

func (z *ioDecReaderCommon) last() byte {
	return z.l
}

func (z *ioDecReaderCommon) reset(r io.Reader, blist *bytesFreelist) {
	z.blist = blist
	z.r = r
	z.ls = unreadByteUndefined
	z.l, z.n = 0, 0
	z.trb = false
}

func (z *ioDecReaderCommon) numread() uint {
	return z.n
}

func (z *ioDecReaderCommon) track() {
	z.tr = z.blist.check(z.tr, 256)[:0]
	z.trb = true
}

func (z *ioDecReaderCommon) stopTrack() (bs []byte) {
	z.trb = false
	return z.tr
}

// ------------------------------------------

// ioDecReader is a decReader that reads off an io.Reader.
//
// It also has a fallback implementation of ByteScanner if needed.
type ioDecReader struct {
	ioDecReaderCommon

	// rr io.Reader
	br io.ByteScanner

	x [64 + 16]byte // for: get struct field name, swallow valueTypeBytes, etc
	// _ [1]uint64                 // padding
}

func (z *ioDecReader) reset(r io.Reader, blist *bytesFreelist) {
	z.ioDecReaderCommon.reset(r, blist)

	z.br, _ = r.(io.ByteScanner)
}

func (z *ioDecReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}
	var firstByte bool
	if z.ls == unreadByteCanRead {
		z.ls = unreadByteCanUnread
		p[0] = z.l
		if len(p) == 1 {
			n = 1
			return
		}
		firstByte = true
		p = p[1:]
	}
	n, err = z.r.Read(p)
	if n > 0 {
		if err == io.EOF && n == len(p) {
			err = nil // read was successful, so postpone EOF (till next time)
		}
		z.l = p[n-1]
		z.ls = unreadByteCanUnread
	}
	if firstByte {
		n++
	}
	return
}

func (z *ioDecReader) ReadByte() (c byte, err error) {
	if z.br != nil {
		c, err = z.br.ReadByte()
		if err == nil {
			z.l = c
			z.ls = unreadByteCanUnread
		}
		return
	}

	n, err := z.Read(z.b[:1])
	if n == 1 {
		c = z.b[0]
		if err == io.EOF {
			err = nil // read was successful, so postpone EOF (till next time)
		}
	}
	return
}

func (z *ioDecReader) UnreadByte() (err error) {
	if z.br != nil {
		err = z.br.UnreadByte()
		if err == nil {
			z.ls = unreadByteCanRead
		}
		return
	}

	switch z.ls {
	case unreadByteCanUnread:
		z.ls = unreadByteCanRead
	case unreadByteCanRead:
		err = errDecUnreadByteLastByteNotRead
	case unreadByteUndefined:
		err = errDecUnreadByteNothingToRead
	default:
		err = errDecUnreadByteUnknown
	}
	return
}

func (z *ioDecReader) readn(num uint8) (bs [rwNLen]byte) {
	z.readb(bs[:num])
	// copy(bs[:], z.readx(uint(num)))
	return
}

func (z *ioDecReader) readx(n uint) (bs []byte) {
	if n == 0 {
		return
	}
	if n < uint(len(z.x)) {
		bs = z.x[:n]
	} else {
		bs = make([]byte, n)
	}
	if _, err := decReadFull(z.r, bs); err != nil {
		panic(err)
	}
	z.n += uint(len(bs))
	if z.trb {
		z.tr = append(z.tr, bs...)
	}
	return
}

func (z *ioDecReader) readb(bs []byte) {
	if len(bs) == 0 {
		return
	}
	if _, err := decReadFull(z.r, bs); err != nil {
		panic(err)
	}
	z.n += uint(len(bs))
	if z.trb {
		z.tr = append(z.tr, bs...)
	}
}

func (z *ioDecReader) readn1eof() (b uint8, eof bool) {
	b, err := z.ReadByte()
	if err == nil {
		z.n++
		if z.trb {
			z.tr = append(z.tr, b)
		}
	} else if err == io.EOF {
		eof = true
	} else {
		panic(err)
	}
	return
}

func (z *ioDecReader) readn1() (b uint8) {
	b, err := z.ReadByte()
	if err == nil {
		z.n++
		if z.trb {
			z.tr = append(z.tr, b)
		}
		return
	}
	panic(err)
}

func (z *ioDecReader) skip(accept *bitset256) (token byte) {
	var eof bool
LOOP:
	token, eof = z.readn1eof()
	if eof {
		return
	}
	if accept.isset(token) {
		goto LOOP
	}
	return
}

func (z *ioDecReader) readTo(accept *bitset256) []byte {
	z.bufr = z.blist.check(z.bufr, 256)[:0]
LOOP:
	token, eof := z.readn1eof()
	if eof {
		return z.bufr
	}
	if accept.isset(token) {
		z.bufr = append(z.bufr, token)
		goto LOOP
	}
	z.unreadn1()
	return z.bufr
}

func (z *ioDecReader) readUntil(stop byte, includeLast bool) []byte {
	z.bufr = z.blist.check(z.bufr, 256)[:0]
LOOP:
	token, eof := z.readn1eof()
	if eof {
		panic(io.EOF)
	}
	z.bufr = append(z.bufr, token)
	if token == stop {
		if includeLast {
			return z.bufr
		}
		return z.bufr[:len(z.bufr)-1]
	}
	goto LOOP
}

//go:noinline
func (z *ioDecReader) unreadn1() {
	err := z.UnreadByte()
	if err != nil {
		panic(err)
	}
	z.n--
	if z.trb {
		if l := len(z.tr) - 1; l >= 0 {
			z.tr = z.tr[:l]
		}
	}
}

// ------------------------------------

type bufioDecReader struct {
	ioDecReaderCommon

	c   uint // cursor
	buf []byte
}

func (z *bufioDecReader) reset(r io.Reader, bufsize int, blist *bytesFreelist) {
	z.ioDecReaderCommon.reset(r, blist)
	z.c = 0
	if cap(z.buf) < bufsize {
		z.buf = blist.get(bufsize)
	}
	z.buf = z.buf[:0]
}

func (z *bufioDecReader) readb(p []byte) {
	var n = uint(copy(p, z.buf[z.c:]))
	z.n += n
	z.c += n
	if len(p) == int(n) {
		if z.trb {
			z.tr = append(z.tr, p...)
		}
	} else {
		z.readbFill(p, n)
	}
}

func (z *bufioDecReader) readbFill(p0 []byte, n uint) {
	// at this point, there's nothing in z.buf to read (z.buf is fully consumed)
	p := p0[n:]
	var n2 uint
	var err error
	if len(p) > cap(z.buf) {
		n2, err = decReadFull(z.r, p)
		if err != nil {
			panic(err)
		}
		n += n2
		z.n += n2
		// always keep last byte in z.buf
		z.buf = z.buf[:1]
		z.buf[0] = p[len(p)-1]
		z.c = 1
		if z.trb {
			z.tr = append(z.tr, p0[:n]...)
		}
		return
	}
	// z.c is now 0, and len(p) <= cap(z.buf)
LOOP:
	// for len(p) > 0 && z.err == nil {
	if len(p) > 0 {
		z.buf = z.buf[0:cap(z.buf)]
		var n1 int
		n1, err = z.r.Read(z.buf)
		n2 = uint(n1)
		if n2 == 0 && err != nil {
			panic(err)
		}
		z.buf = z.buf[:n2]
		n2 = uint(copy(p, z.buf))
		z.c = n2
		n += n2
		z.n += n2
		p = p[n2:]
		goto LOOP
	}
	if z.c == 0 {
		z.buf = z.buf[:1]
		z.buf[0] = p[len(p)-1]
		z.c = 1
	}
	if z.trb {
		z.tr = append(z.tr, p0[:n]...)
	}
}

func (z *bufioDecReader) last() byte {
	return z.buf[z.c-1]
}

func (z *bufioDecReader) readn1() (b byte) {
	// fast-path, so we elide calling into Read() most of the time
	if z.c < uint(len(z.buf)) {
		b = z.buf[z.c]
		z.c++
		z.n++
		if z.trb {
			z.tr = append(z.tr, b)
		}
	} else { // meaning z.c == len(z.buf) or greater ... so need to fill
		z.readbFill(z.b[:1], 0)
		b = z.b[0]
	}
	return
}

func (z *bufioDecReader) unreadn1() {
	if z.c == 0 {
		panic(errDecUnreadByteNothingToRead)
	}
	z.c--
	z.n--
	if z.trb {
		z.tr = z.tr[:len(z.tr)-1]
	}
}

func (z *bufioDecReader) readn(num uint8) (bs [rwNLen]byte) {
	z.readb(bs[:num])
	// copy(bs[:], z.readx(uint(num)))
	return
}

func (z *bufioDecReader) readx(n uint) (bs []byte) {
	if n == 0 {
		// return
	} else if z.c+n <= uint(len(z.buf)) {
		bs = z.buf[z.c : z.c+n]
		z.n += n
		z.c += n
		if z.trb {
			z.tr = append(z.tr, bs...)
		}
	} else {
		bs = make([]byte, n)
		// n no longer used - can reuse
		n = uint(copy(bs, z.buf[z.c:]))
		z.n += n
		z.c += n
		z.readbFill(bs, n)
	}
	return
}

func (z *bufioDecReader) skip(accept *bitset256) (token byte) {
	i := z.c
LOOP:
	if i < uint(len(z.buf)) {
		// inline z.skipLoopFn(i) and refactor, so cost is within inline budget
		token = z.buf[i]
		i++
		if accept.isset(token) {
			goto LOOP
		}
		z.n += i - 2 - z.c
		if z.trb {
			z.tr = append(z.tr, z.buf[z.c:i]...) // z.doTrack(i)
		}
		z.c = i
		return
	}
	return z.skipFill(accept)
}

func (z *bufioDecReader) skipFill(accept *bitset256) (token byte) {
	z.n += uint(len(z.buf)) - z.c
	if z.trb {
		z.tr = append(z.tr, z.buf[z.c:]...)
	}
	var i, n2 int
	var err error
	for {
		z.c = 0
		z.buf = z.buf[0:cap(z.buf)]
		n2, err = z.r.Read(z.buf)
		if n2 == 0 && err != nil {
			panic(err)
		}
		z.buf = z.buf[:n2]
		for i, token = range z.buf {
			// if !accept.isset(token) {
			if accept.check(token) == 0 {
				z.n += (uint(i) - z.c) - 1
				z.loopFn(uint(i + 1))
				return
			}
		}
		z.n += uint(n2)
		if z.trb {
			z.tr = append(z.tr, z.buf...)
		}
	}
}

func (z *bufioDecReader) loopFn(i uint) {
	if z.trb {
		z.tr = append(z.tr, z.buf[z.c:i]...) // z.doTrack(i)
	}
	z.c = i
}

func (z *bufioDecReader) readTo(accept *bitset256) (out []byte) {
	i := z.c
LOOP:
	if i < uint(len(z.buf)) {
		// if !accept.isset(z.buf[i]) {
		if accept.check(z.buf[i]) == 0 {
			// inline readToLoopFn here (for performance)
			z.n += (i - z.c) - 1
			out = z.buf[z.c:i]
			if z.trb {
				z.tr = append(z.tr, z.buf[z.c:i]...) // z.doTrack(i)
			}
			z.c = i
			return
		}
		i++
		goto LOOP
	}
	return z.readToFill(accept)
}

func (z *bufioDecReader) readToFill(accept *bitset256) []byte {
	z.bufr = z.blist.check(z.bufr, 256)[:0]
	z.n += uint(len(z.buf)) - z.c
	z.bufr = append(z.bufr, z.buf[z.c:]...)
	if z.trb {
		z.tr = append(z.tr, z.buf[z.c:]...)
	}
	var n2 int
	var err error
	for {
		z.c = 0
		z.buf = z.buf[:cap(z.buf)]
		n2, err = z.r.Read(z.buf)
		if n2 == 0 && err != nil {
			if err == io.EOF {
				return z.bufr // readTo should read until it matches or end is reached
			}
			panic(err)
		}
		z.buf = z.buf[:n2]
		for i, token := range z.buf {
			// if !accept.isset(token) {
			if accept.check(token) == 0 {
				z.n += (uint(i) - z.c) - 1
				z.bufr = append(z.bufr, z.buf[z.c:i]...)
				z.loopFn(uint(i))
				return z.bufr
			}
		}
		z.bufr = append(z.bufr, z.buf...)
		z.n += uint(n2)
		if z.trb {
			z.tr = append(z.tr, z.buf...)
		}
	}
}

func (z *bufioDecReader) readUntil(stop byte, includeLast bool) (out []byte) {
	i := z.c
LOOP:
	if i < uint(len(z.buf)) {
		if z.buf[i] == stop {
			z.n += (i - z.c) - 1
			i++
			out = z.buf[z.c:i]
			if z.trb {
				z.tr = append(z.tr, z.buf[z.c:i]...) // z.doTrack(i)
			}
			z.c = i
			goto FINISH
		}
		i++
		goto LOOP
	}
	out = z.readUntilFill(stop)
FINISH:
	if includeLast {
		return
	}
	return out[:len(out)-1]
}

func (z *bufioDecReader) readUntilFill(stop byte) []byte {
	z.bufr = z.blist.check(z.bufr, 256)[:0]
	z.n += uint(len(z.buf)) - z.c
	z.bufr = append(z.bufr, z.buf[z.c:]...)
	if z.trb {
		z.tr = append(z.tr, z.buf[z.c:]...)
	}
	for {
		z.c = 0
		z.buf = z.buf[0:cap(z.buf)]
		n1, err := z.r.Read(z.buf)
		if n1 == 0 && err != nil {
			panic(err)
		}
		n2 := uint(n1)
		z.buf = z.buf[:n2]
		for i, token := range z.buf {
			if token == stop {
				z.n += (uint(i) - z.c) - 1
				z.bufr = append(z.bufr, z.buf[z.c:i+1]...)
				z.loopFn(uint(i + 1))
				return z.bufr
			}
		}
		z.bufr = append(z.bufr, z.buf...)
		z.n += n2
		if z.trb {
			z.tr = append(z.tr, z.buf...)
		}
	}
}

// ------------------------------------

// bytesDecReader is a decReader that reads off a byte slice with zero copying
type bytesDecReader struct {
	b []byte // data
	c uint   // cursor
	t uint   // track start
	// a int    // available
}

func (z *bytesDecReader) reset(in []byte) {
	z.b = in
	z.c = 0
	z.t = 0
}

func (z *bytesDecReader) numread() uint {
	return z.c
}

func (z *bytesDecReader) last() byte {
	return z.b[z.c-1]
}

func (z *bytesDecReader) unreadn1() {
	if z.c == 0 || len(z.b) == 0 {
		panic(errBytesDecReaderCannotUnread)
	}
	z.c--
}

func (z *bytesDecReader) readx(n uint) (bs []byte) {
	// slicing from a non-constant start position is more expensive,
	// as more computation is required to decipher the pointer start position.
	// However, we do it only once, and it's better than reslicing both z.b and return value.

	z.c += n
	return z.b[z.c-n : z.c]
}

func (z *bytesDecReader) readb(bs []byte) {
	copy(bs, z.readx(uint(len(bs))))
}

func (z *bytesDecReader) readn1() (v uint8) {
	v = z.b[z.c]
	z.c++
	return
}

func (z *bytesDecReader) readn(num uint8) (bs [rwNLen]byte) {
	// if z.c >= uint(len(z.b)) || z.c+uint(num) >= uint(len(z.b)) {
	// 	panic(io.EOF)
	// }

	// for bounds-check elimination, reslice z.b and ensure bs is within len
	// bb := z.b[z.c:][:num]
	bb := z.b[z.c : z.c+uint(num)]
	_ = bs[len(bb)-1]
	var i int
LOOP:
	if i < len(bb) {
		bs[i] = bb[i]
		i++
		goto LOOP
	}

	z.c += uint(num)
	return
}

func (z *bytesDecReader) skip(accept *bitset256) (token byte) {
	i := z.c
LOOP:
	// if i < uint(len(z.b)) {
	token = z.b[i]
	i++
	if accept.isset(token) {
		goto LOOP
	}
	z.c = i
	return
}

func (z *bytesDecReader) readTo(accept *bitset256) (out []byte) {
	i := z.c
LOOP:
	if i < uint(len(z.b)) {
		if accept.isset(z.b[i]) {
			i++
			goto LOOP
		}
	}

	out = z.b[z.c:i]
	z.c = i
	return // z.b[c:i]
}

func (z *bytesDecReader) readUntil(stop byte, includeLast bool) (out []byte) {
	i := z.c
LOOP:
	// if i < uint(len(z.b)) {
	if z.b[i] == stop {
		i++
		if includeLast {
			out = z.b[z.c:i]
		} else {
			out = z.b[z.c : i-1]
		}
		// z.a -= (i - z.c)
		z.c = i
		return
	}
	i++
	goto LOOP
	// }
	// panic(io.EOF)
}

func (z *bytesDecReader) track() {
	z.t = z.c
}

func (z *bytesDecReader) stopTrack() (bs []byte) {
	return z.b[z.t:z.c]
}

// --------------

type decRd struct {
	mtr bool // is maptype a known type?
	str bool // is slicetype a known type?

	be   bool // is binary encoding
	js   bool // is json handle
	jsms bool // is json handle, and MapKeyAsString
	cbor bool // is cbor handle

	bytes bool // is bytes reader
	bufio bool // is this a bufioDecReader?

	rb bytesDecReader
	ri *ioDecReader
	bi *bufioDecReader
}

// numread, track and stopTrack are always inlined, as they just check int fields, etc.

// the if/else-if/else block is expensive to inline.
// Each node of this construct costs a lot and dominates the budget.
// Best to only do an if fast-path else block (so fast-path is inlined).
// This is irrespective of inlineExtraCallCost set in $GOROOT/src/cmd/compile/internal/gc/inl.go
//
// In decRd methods below, we delegate all IO functions into their own methods.
// This allows for the inlining of the common path when z.bytes=true.
// Go 1.12+ supports inlining methods with up to 1 inlined function (or 2 if no other constructs).
//
// However, up through Go 1.13, decRd's readXXX, skip and unreadXXX methods are not inlined.
// Consequently, there is no benefit to do the xxxIO methods for decRd at this time.
// Instead, we have a if/else-if/else block so that IO calls do not have to jump through
// a second unnecessary function call.
//
// If golang inlining gets better and bytesDecReader methods can be inlined,
// then we can revert to using these 2 functions so the bytesDecReader
// methods are inlined and the IO paths call out to a function.

func (z *decRd) numread() uint {
	if z.bytes {
		return z.rb.numread()
	} else if z.bufio {
		return z.bi.numread()
	} else {
		return z.ri.numread()
	}
}
func (z *decRd) stopTrack() []byte {
	if z.bytes {
		return z.rb.stopTrack()
	} else if z.bufio {
		return z.bi.stopTrack()
	} else {
		return z.ri.stopTrack()
	}
}

func (z *decRd) track() {
	if z.bytes {
		z.rb.track()
	} else if z.bufio {
		z.bi.track()
	} else {
		z.ri.track()
	}
}

func (z *decRd) unreadn1() {
	if z.bytes {
		z.rb.unreadn1()
	} else if z.bufio {
		z.bi.unreadn1()
	} else {
		z.ri.unreadn1() // not inlined
	}
}

func (z *decRd) readn(num uint8) [rwNLen]byte {
	if z.bytes {
		return z.rb.readn(num)
	} else if z.bufio {
		return z.bi.readn(num)
	} else {
		return z.ri.readn(num)
	}
}

func (z *decRd) readx(n uint) []byte {
	if z.bytes {
		return z.rb.readx(n)
	} else if z.bufio {
		return z.bi.readx(n)
	} else {
		return z.ri.readx(n)
	}
}

func (z *decRd) readb(s []byte) {
	if z.bytes {
		z.rb.readb(s)
	} else if z.bufio {
		z.bi.readb(s)
	} else {
		z.ri.readb(s)
	}
}

func (z *decRd) readn1() uint8 {
	if z.bytes {
		return z.rb.readn1()
	} else if z.bufio {
		return z.bi.readn1()
	} else {
		return z.ri.readn1()
	}
}

func (z *decRd) skip(accept *bitset256) (token byte) {
	if z.bytes {
		return z.rb.skip(accept)
	} else if z.bufio {
		return z.bi.skip(accept)
	} else {
		return z.ri.skip(accept)
	}
}

func (z *decRd) readTo(accept *bitset256) (out []byte) {
	if z.bytes {
		return z.rb.readTo(accept)
	} else if z.bufio {
		return z.bi.readTo(accept)
	} else {
		return z.ri.readTo(accept)
	}
}

func (z *decRd) readUntil(stop byte, includeLast bool) (out []byte) {
	if z.bytes {
		return z.rb.readUntil(stop, includeLast)
	} else if z.bufio {
		return z.bi.readUntil(stop, includeLast)
	} else {
		return z.ri.readUntil(stop, includeLast)
	}
}

/*
func (z *decRd) track() {
	if z.bytes {
		z.rb.track()
	} else {
		z.trackIO()
	}
}
func (z *decRd) trackIO() {
	if z.bufio {
		z.bi.track()
	} else {
		z.ri.track()
	}
}

func (z *decRd) unreadn1() {
	if z.bytes {
		z.rb.unreadn1()
	} else {
		z.unreadn1IO()
	}
}
func (z *decRd) unreadn1IO() {
	if z.bufio {
		z.bi.unreadn1()
	} else {
		z.ri.unreadn1()
	}
}

func (z *decRd) readn(num uint8) [rwNLen]byte {
	if z.bytes {
		return z.rb.readn(num)
	}
	return z.readnIO(num)
}
func (z *decRd) readnIO(num uint8) [rwNLen]byte {
	if z.bufio {
		return z.bi.readn(num)
	}
	return z.ri.readn(num)
}

func (z *decRd) readx(n uint) []byte {
	if z.bytes {
		return z.rb.readx(n)
	}
	return z.readxIO(n)
}
func (z *decRd) readxIO(n uint) []byte {
	if z.bufio {
		return z.bi.readx(n)
	}
	return z.ri.readx(n)
}

func (z *decRd) readb(s []byte) {
	if z.bytes {
		z.rb.readb(s)
	} else {
		z.readbIO(s)
	}
}
func (z *decRd) readbIO(s []byte) {
	if z.bufio {
		z.bi.readb(s)
	} else {
		z.ri.readb(s)
	}
}

func (z *decRd) readn1() uint8 {
	if z.bytes {
		return z.rb.readn1()
	}
	return z.readn1IO()
}
func (z *decRd) readn1IO() uint8 {
	if z.bufio {
		return z.bi.readn1()
	}
	return z.ri.readn1()
}

func (z *decRd) skip(accept *bitset256) (token byte) {
	if z.bytes {
		return z.rb.skip(accept)
	}
	return z.skipIO(accept)
}
func (z *decRd) skipIO(accept *bitset256) (token byte) {
	if z.bufio {
		return z.bi.skip(accept)
	}
	return z.ri.skip(accept)
}

func (z *decRd) readTo(accept *bitset256) (out []byte) {
	if z.bytes {
		return z.rb.readTo(accept)
	}
	return z.readToIO(accept)
}
func (z *decRd) readToIO(accept *bitset256) (out []byte) {
	if z.bufio {
		return z.bi.readTo(accept)
	}
	return z.ri.readTo(accept)
}

func (z *decRd) readUntil(stop byte, includeLast bool) (out []byte) {
	if z.bytes {
		return z.rb.readUntil(stop, includeLast)
	}
	return z.readUntilIO(stop, includeLast)
}
func (z *decRd) readUntilIO(stop byte, includeLast bool) (out []byte) {
	if z.bufio {
		return z.bi.readUntil(stop, includeLast)
	}
	return z.ri.readUntil(stop, includeLast)
}
*/

var _ decReader = (*decRd)(nil)
