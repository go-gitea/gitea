package xml

import "github.com/tdewolff/parse/v2/xml"

// Token is a single token unit with an attribute value (if given) and hash of the data.
type Token struct {
	xml.TokenType
	Data    []byte
	Text    []byte
	AttrVal []byte
}

// TokenBuffer is a buffer that allows for token look-ahead.
type TokenBuffer struct {
	l *xml.Lexer

	buf []Token
	pos int
}

// NewTokenBuffer returns a new TokenBuffer.
func NewTokenBuffer(l *xml.Lexer) *TokenBuffer {
	return &TokenBuffer{
		l:   l,
		buf: make([]Token, 0, 8),
	}
}

func (z *TokenBuffer) read(t *Token) {
	t.TokenType, t.Data = z.l.Next()
	t.Text = z.l.Text()
	if t.TokenType == xml.AttributeToken {
		t.AttrVal = z.l.AttrVal()
	} else {
		t.AttrVal = nil
	}
}

// Peek returns the ith element and possibly does an allocation.
// Peeking past an error will panic.
func (z *TokenBuffer) Peek(pos int) *Token {
	pos += z.pos
	if pos >= len(z.buf) {
		if len(z.buf) > 0 && z.buf[len(z.buf)-1].TokenType == xml.ErrorToken {
			return &z.buf[len(z.buf)-1]
		}

		c := cap(z.buf)
		d := len(z.buf) - z.pos
		p := pos - z.pos + 1 // required peek length
		var buf []Token
		if 2*p > c {
			buf = make([]Token, 0, 2*c+p)
		} else {
			buf = z.buf
		}
		copy(buf[:d], z.buf[z.pos:])

		buf = buf[:p]
		pos -= z.pos
		for i := d; i < p; i++ {
			z.read(&buf[i])
			if buf[i].TokenType == xml.ErrorToken {
				buf = buf[:i+1]
				pos = i
				break
			}
		}
		z.pos, z.buf = 0, buf
	}
	return &z.buf[pos]
}

// Shift returns the first element and advances position.
func (z *TokenBuffer) Shift() *Token {
	if z.pos >= len(z.buf) {
		t := &z.buf[:1][0]
		z.read(t)
		return t
	}
	t := &z.buf[z.pos]
	z.pos++
	return t
}
