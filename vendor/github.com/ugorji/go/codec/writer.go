// Copyright (c) 2012-2018 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

import "io"

// encWriter abstracts writing to a byte array or to an io.Writer.
type encWriter interface {
	writeb([]byte)
	writestr(string)
	writeqstr(string) // write string wrapped in quotes ie "..."
	writen1(byte)
	writen2(byte, byte)
	// writen will write up to 7 bytes at a time.
	writen(b [rwNLen]byte, num uint8)
	end()
}

// ---------------------------------------------

// bufioEncWriter
type bufioEncWriter struct {
	w io.Writer

	buf []byte

	n int

	b [16]byte // scratch buffer and padding (cache-aligned)
}

func (z *bufioEncWriter) reset(w io.Writer, bufsize int, blist *bytesFreelist) {
	z.w = w
	z.n = 0
	if bufsize <= 0 {
		bufsize = defEncByteBufSize
	}
	// bufsize must be >= 8, to accomodate writen methods (where n <= 8)
	if bufsize <= 8 {
		bufsize = 8
	}
	if cap(z.buf) < bufsize {
		if len(z.buf) > 0 && &z.buf[0] != &z.b[0] {
			blist.put(z.buf)
		}
		if len(z.b) > bufsize {
			z.buf = z.b[:]
		} else {
			z.buf = blist.get(bufsize)
		}
	}
	z.buf = z.buf[:cap(z.buf)]
}

//go:noinline - flush only called intermittently
func (z *bufioEncWriter) flushErr() (err error) {
	n, err := z.w.Write(z.buf[:z.n])
	z.n -= n
	if z.n > 0 && err == nil {
		err = io.ErrShortWrite
	}
	if n > 0 && z.n > 0 {
		copy(z.buf, z.buf[n:z.n+n])
	}
	return err
}

func (z *bufioEncWriter) flush() {
	if err := z.flushErr(); err != nil {
		panic(err)
	}
}

func (z *bufioEncWriter) writeb(s []byte) {
LOOP:
	a := len(z.buf) - z.n
	if len(s) > a {
		z.n += copy(z.buf[z.n:], s[:a])
		s = s[a:]
		z.flush()
		goto LOOP
	}
	z.n += copy(z.buf[z.n:], s)
}

func (z *bufioEncWriter) writestr(s string) {
	// z.writeb(bytesView(s)) // inlined below
LOOP:
	a := len(z.buf) - z.n
	if len(s) > a {
		z.n += copy(z.buf[z.n:], s[:a])
		s = s[a:]
		z.flush()
		goto LOOP
	}
	z.n += copy(z.buf[z.n:], s)
}

func (z *bufioEncWriter) writeqstr(s string) {
	// z.writen1('"')
	// z.writestr(s)
	// z.writen1('"')

	if z.n+len(s)+2 > len(z.buf) {
		z.flush()
	}
	z.buf[z.n] = '"'
	z.n++
LOOP:
	a := len(z.buf) - z.n
	if len(s)+1 > a {
		z.n += copy(z.buf[z.n:], s[:a])
		s = s[a:]
		z.flush()
		goto LOOP
	}
	z.n += copy(z.buf[z.n:], s)
	z.buf[z.n] = '"'
	z.n++
}

func (z *bufioEncWriter) writen1(b1 byte) {
	if 1 > len(z.buf)-z.n {
		z.flush()
	}
	z.buf[z.n] = b1
	z.n++
}

func (z *bufioEncWriter) writen2(b1, b2 byte) {
	if 2 > len(z.buf)-z.n {
		z.flush()
	}
	z.buf[z.n+1] = b2
	z.buf[z.n] = b1
	z.n += 2
}

func (z *bufioEncWriter) writen(b [rwNLen]byte, num uint8) {
	if int(num) > len(z.buf)-z.n {
		z.flush()
	}
	copy(z.buf[z.n:], b[:num])
	z.n += int(num)
}

func (z *bufioEncWriter) endErr() (err error) {
	if z.n > 0 {
		err = z.flushErr()
	}
	return
}

// ---------------------------------------------

// bytesEncAppender implements encWriter and can write to an byte slice.
type bytesEncAppender struct {
	b   []byte
	out *[]byte
}

func (z *bytesEncAppender) writeb(s []byte) {
	z.b = append(z.b, s...)
}
func (z *bytesEncAppender) writestr(s string) {
	z.b = append(z.b, s...)
}
func (z *bytesEncAppender) writeqstr(s string) {
	z.b = append(append(append(z.b, '"'), s...), '"')

	// z.b = append(z.b, '"')
	// z.b = append(z.b, s...)
	// z.b = append(z.b, '"')
}
func (z *bytesEncAppender) writen1(b1 byte) {
	z.b = append(z.b, b1)
}
func (z *bytesEncAppender) writen2(b1, b2 byte) {
	z.b = append(z.b, b1, b2) // cost: 81
}
func (z *bytesEncAppender) writen(s [rwNLen]byte, num uint8) {
	// if num <= rwNLen {
	if int(num) <= len(s) {
		z.b = append(z.b, s[:num]...)
	}
}
func (z *bytesEncAppender) endErr() error {
	*(z.out) = z.b
	return nil
}
func (z *bytesEncAppender) reset(in []byte, out *[]byte) {
	z.b = in[:0]
	z.out = out
}

// --------------------------------------------------

type encWr struct {
	bytes bool // encoding to []byte
	js    bool // is json encoder?
	be    bool // is binary encoder?

	c containerState

	calls uint16

	wb bytesEncAppender
	wf *bufioEncWriter
}

func (z *encWr) writeb(s []byte) {
	if z.bytes {
		z.wb.writeb(s)
	} else {
		z.wf.writeb(s)
	}
}
func (z *encWr) writeqstr(s string) {
	if z.bytes {
		z.wb.writeqstr(s)
	} else {
		z.wf.writeqstr(s)
	}
}
func (z *encWr) writestr(s string) {
	if z.bytes {
		z.wb.writestr(s)
	} else {
		z.wf.writestr(s)
	}
}
func (z *encWr) writen1(b1 byte) {
	if z.bytes {
		z.wb.writen1(b1)
	} else {
		z.wf.writen1(b1)
	}
}
func (z *encWr) writen2(b1, b2 byte) {
	if z.bytes {
		z.wb.writen2(b1, b2)
	} else {
		z.wf.writen2(b1, b2)
	}
}
func (z *encWr) writen(b [rwNLen]byte, num uint8) {
	if z.bytes {
		z.wb.writen(b, num)
	} else {
		z.wf.writen(b, num)
	}
}
func (z *encWr) endErr() error {
	if z.bytes {
		return z.wb.endErr()
	}
	return z.wf.endErr()
}

func (z *encWr) end() {
	if err := z.endErr(); err != nil {
		panic(err)
	}
}

var _ encWriter = (*encWr)(nil)
