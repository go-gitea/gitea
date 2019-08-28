package buffer

import (
	"io"
	"io/ioutil"
)

var nullBuffer = []byte{0}

// Lexer is a buffered reader that allows peeking forward and shifting, taking an io.Reader.
// It keeps data in-memory until Free, taking a byte length, is called to move beyond the data.
type Lexer struct {
	buf   []byte
	pos   int // index in buf
	start int // index in buf
	err   error

	restore func()
}

// NewLexerBytes returns a new Lexer for a given io.Reader, and uses ioutil.ReadAll to read it into a byte slice.
// If the io.Reader implements Bytes, that is used instead.
// It will append a NULL at the end of the buffer.
func NewLexer(r io.Reader) *Lexer {
	var b []byte
	if r != nil {
		if buffer, ok := r.(interface {
			Bytes() []byte
		}); ok {
			b = buffer.Bytes()
		} else {
			var err error
			b, err = ioutil.ReadAll(r)
			if err != nil {
				return &Lexer{
					buf: []byte{0},
					err: err,
				}
			}
		}
	}
	return NewLexerBytes(b)
}

// NewLexerBytes returns a new Lexer for a given byte slice, and appends NULL at the end.
// To avoid reallocation, make sure the capacity has room for one more byte.
func NewLexerBytes(b []byte) *Lexer {
	z := &Lexer{
		buf: b,
	}

	n := len(b)
	if n == 0 {
		z.buf = nullBuffer
	} else if b[n-1] != 0 {
		// Append NULL to buffer, but try to avoid reallocation
		if cap(b) > n {
			// Overwrite next byte but restore when done
			b = b[:n+1]
			c := b[n]
			b[n] = 0

			z.buf = b
			z.restore = func() {
				b[n] = c
			}
		} else {
			z.buf = append(b, 0)
		}
	}
	return z
}

// Restore restores the replaced byte past the end of the buffer by NULL.
func (z *Lexer) Restore() {
	if z.restore != nil {
		z.restore()
		z.restore = nil
	}
}

// Err returns the error returned from io.Reader or io.EOF when the end has been reached.
func (z *Lexer) Err() error {
	return z.PeekErr(0)
}

// PeekErr returns the error at position pos. When pos is zero, this is the same as calling Err().
func (z *Lexer) PeekErr(pos int) error {
	if z.err != nil {
		return z.err
	} else if z.pos+pos >= len(z.buf)-1 {
		return io.EOF
	}
	return nil
}

// Peek returns the ith byte relative to the end position.
// Peek returns 0 when an error has occurred, Err returns the error.
func (z *Lexer) Peek(pos int) byte {
	pos += z.pos
	return z.buf[pos]
}

// PeekRune returns the rune and rune length of the ith byte relative to the end position.
func (z *Lexer) PeekRune(pos int) (rune, int) {
	// from unicode/utf8
	c := z.Peek(pos)
	if c < 0xC0 || z.Peek(pos+1) == 0 {
		return rune(c), 1
	} else if c < 0xE0 || z.Peek(pos+2) == 0 {
		return rune(c&0x1F)<<6 | rune(z.Peek(pos+1)&0x3F), 2
	} else if c < 0xF0 || z.Peek(pos+3) == 0 {
		return rune(c&0x0F)<<12 | rune(z.Peek(pos+1)&0x3F)<<6 | rune(z.Peek(pos+2)&0x3F), 3
	}
	return rune(c&0x07)<<18 | rune(z.Peek(pos+1)&0x3F)<<12 | rune(z.Peek(pos+2)&0x3F)<<6 | rune(z.Peek(pos+3)&0x3F), 4
}

// Move advances the position.
func (z *Lexer) Move(n int) {
	z.pos += n
}

// Pos returns a mark to which can be rewinded.
func (z *Lexer) Pos() int {
	return z.pos - z.start
}

// Rewind rewinds the position to the given position.
func (z *Lexer) Rewind(pos int) {
	z.pos = z.start + pos
}

// Lexeme returns the bytes of the current selection.
func (z *Lexer) Lexeme() []byte {
	return z.buf[z.start:z.pos]
}

// Skip collapses the position to the end of the selection.
func (z *Lexer) Skip() {
	z.start = z.pos
}

// Shift returns the bytes of the current selection and collapses the position to the end of the selection.
func (z *Lexer) Shift() []byte {
	b := z.buf[z.start:z.pos]
	z.start = z.pos
	return b
}

// Offset returns the character position in the buffer.
func (z *Lexer) Offset() int {
	return z.pos
}

// Bytes returns the underlying buffer.
func (z *Lexer) Bytes() []byte {
	return z.buf[:len(z.buf)-1]
}
