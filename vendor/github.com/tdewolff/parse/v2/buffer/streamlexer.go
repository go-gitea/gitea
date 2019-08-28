package buffer

import (
	"io"
)

type block struct {
	buf    []byte
	next   int // index in pool plus one
	active bool
}

type bufferPool struct {
	pool []block
	head int // index in pool plus one
	tail int // index in pool plus one

	pos int // byte pos in tail
}

func (z *bufferPool) swap(oldBuf []byte, size int) []byte {
	// find new buffer that can be reused
	swap := -1
	for i := 0; i < len(z.pool); i++ {
		if !z.pool[i].active && size <= cap(z.pool[i].buf) {
			swap = i
			break
		}
	}
	if swap == -1 { // no free buffer found for reuse
		if z.tail == 0 && z.pos >= len(oldBuf) && size <= cap(oldBuf) { // but we can reuse the current buffer!
			z.pos -= len(oldBuf)
			return oldBuf[:0]
		}
		// allocate new
		z.pool = append(z.pool, block{make([]byte, 0, size), 0, true})
		swap = len(z.pool) - 1
	}

	newBuf := z.pool[swap].buf

	// put current buffer into pool
	z.pool[swap] = block{oldBuf, 0, true}
	if z.head != 0 {
		z.pool[z.head-1].next = swap + 1
	}
	z.head = swap + 1
	if z.tail == 0 {
		z.tail = swap + 1
	}

	return newBuf[:0]
}

func (z *bufferPool) free(n int) {
	z.pos += n
	// move the tail over to next buffers
	for z.tail != 0 && z.pos >= len(z.pool[z.tail-1].buf) {
		z.pos -= len(z.pool[z.tail-1].buf)
		newTail := z.pool[z.tail-1].next
		z.pool[z.tail-1].active = false // after this, any thread may pick up the inactive buffer, so it can't be used anymore
		z.tail = newTail
	}
	if z.tail == 0 {
		z.head = 0
	}
}

// StreamLexer is a buffered reader that allows peeking forward and shifting, taking an io.Reader.
// It keeps data in-memory until Free, taking a byte length, is called to move beyond the data.
type StreamLexer struct {
	r   io.Reader
	err error

	pool bufferPool

	buf       []byte
	start     int // index in buf
	pos       int // index in buf
	prevStart int

	free int
}

// NewStreamLexer returns a new StreamLexer for a given io.Reader with a 4kB estimated buffer size.
// If the io.Reader implements Bytes, that buffer is used instead.
func NewStreamLexer(r io.Reader) *StreamLexer {
	return NewStreamLexerSize(r, defaultBufSize)
}

// NewStreamLexerSize returns a new StreamLexer for a given io.Reader and estimated required buffer size.
// If the io.Reader implements Bytes, that buffer is used instead.
func NewStreamLexerSize(r io.Reader, size int) *StreamLexer {
	// if reader has the bytes in memory already, use that instead
	if buffer, ok := r.(interface {
		Bytes() []byte
	}); ok {
		return &StreamLexer{
			err: io.EOF,
			buf: buffer.Bytes(),
		}
	}
	return &StreamLexer{
		r:   r,
		buf: make([]byte, 0, size),
	}
}

func (z *StreamLexer) read(pos int) byte {
	if z.err != nil {
		return 0
	}

	// free unused bytes
	z.pool.free(z.free)
	z.free = 0

	// get new buffer
	c := cap(z.buf)
	p := pos - z.start + 1
	if 2*p > c { // if the token is larger than half the buffer, increase buffer size
		c = 2*c + p
	}
	d := len(z.buf) - z.start
	buf := z.pool.swap(z.buf[:z.start], c)
	copy(buf[:d], z.buf[z.start:]) // copy the left-overs (unfinished token) from the old buffer

	// read in new data for the rest of the buffer
	var n int
	for pos-z.start >= d && z.err == nil {
		n, z.err = z.r.Read(buf[d:cap(buf)])
		d += n
	}
	pos -= z.start
	z.pos -= z.start
	z.start, z.buf = 0, buf[:d]
	if pos >= d {
		return 0
	}
	return z.buf[pos]
}

// Err returns the error returned from io.Reader. It may still return valid bytes for a while though.
func (z *StreamLexer) Err() error {
	if z.err == io.EOF && z.pos < len(z.buf) {
		return nil
	}
	return z.err
}

// Free frees up bytes of length n from previously shifted tokens.
// Each call to Shift should at one point be followed by a call to Free with a length returned by ShiftLen.
func (z *StreamLexer) Free(n int) {
	z.free += n
}

// Peek returns the ith byte relative to the end position and possibly does an allocation.
// Peek returns zero when an error has occurred, Err returns the error.
// TODO: inline function
func (z *StreamLexer) Peek(pos int) byte {
	pos += z.pos
	if uint(pos) < uint(len(z.buf)) { // uint for BCE
		return z.buf[pos]
	}
	return z.read(pos)
}

// PeekRune returns the rune and rune length of the ith byte relative to the end position.
func (z *StreamLexer) PeekRune(pos int) (rune, int) {
	// from unicode/utf8
	c := z.Peek(pos)
	if c < 0xC0 {
		return rune(c), 1
	} else if c < 0xE0 {
		return rune(c&0x1F)<<6 | rune(z.Peek(pos+1)&0x3F), 2
	} else if c < 0xF0 {
		return rune(c&0x0F)<<12 | rune(z.Peek(pos+1)&0x3F)<<6 | rune(z.Peek(pos+2)&0x3F), 3
	}
	return rune(c&0x07)<<18 | rune(z.Peek(pos+1)&0x3F)<<12 | rune(z.Peek(pos+2)&0x3F)<<6 | rune(z.Peek(pos+3)&0x3F), 4
}

// Move advances the position.
func (z *StreamLexer) Move(n int) {
	z.pos += n
}

// Pos returns a mark to which can be rewinded.
func (z *StreamLexer) Pos() int {
	return z.pos - z.start
}

// Rewind rewinds the position to the given position.
func (z *StreamLexer) Rewind(pos int) {
	z.pos = z.start + pos
}

// Lexeme returns the bytes of the current selection.
func (z *StreamLexer) Lexeme() []byte {
	return z.buf[z.start:z.pos]
}

// Skip collapses the position to the end of the selection.
func (z *StreamLexer) Skip() {
	z.start = z.pos
}

// Shift returns the bytes of the current selection and collapses the position to the end of the selection.
// It also returns the number of bytes we moved since the last call to Shift. This can be used in calls to Free.
func (z *StreamLexer) Shift() []byte {
	if z.pos > len(z.buf) { // make sure we peeked at least as much as we shift
		z.read(z.pos - 1)
	}
	b := z.buf[z.start:z.pos]
	z.start = z.pos
	return b
}

// ShiftLen returns the number of bytes moved since the last call to ShiftLen. This can be used in calls to Free because it takes into account multiple Shifts or Skips.
func (z *StreamLexer) ShiftLen() int {
	n := z.start - z.prevStart
	z.prevStart = z.start
	return n
}
