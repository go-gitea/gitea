package svg

import (
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/svg"
	"github.com/tdewolff/parse/v2/xml"
)

// Token is a single token unit with an attribute value (if given) and hash of the data.
type Token struct {
	xml.TokenType
	Hash    svg.Hash
	Data    []byte
	Text    []byte
	AttrVal []byte
}

// TokenBuffer is a buffer that allows for token look-ahead.
type TokenBuffer struct {
	l *xml.Lexer

	buf []Token
	pos int

	attrBuffer []*Token
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
		if len(t.AttrVal) > 1 && (t.AttrVal[0] == '"' || t.AttrVal[0] == '\'') {
			t.AttrVal = parse.ReplaceMultipleWhitespace(parse.TrimWhitespace(t.AttrVal[1 : len(t.AttrVal)-1])) // quotes will be readded in attribute loop if necessary
		}
		t.Hash = svg.ToHash(t.Text)
	} else if t.TokenType == xml.StartTagToken || t.TokenType == xml.EndTagToken {
		t.AttrVal = nil
		t.Hash = svg.ToHash(t.Text)
	} else {
		t.AttrVal = nil
		t.Hash = 0
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

// Attributes extracts the gives attribute hashes from a tag.
// It returns in the same order pointers to the requested token data or nil.
func (z *TokenBuffer) Attributes(hashes ...svg.Hash) ([]*Token, *Token) {
	n := 0
	for {
		if t := z.Peek(n); t.TokenType != xml.AttributeToken {
			break
		}
		n++
	}
	if len(hashes) > cap(z.attrBuffer) {
		z.attrBuffer = make([]*Token, len(hashes))
	} else {
		z.attrBuffer = z.attrBuffer[:len(hashes)]
		for i := range z.attrBuffer {
			z.attrBuffer[i] = nil
		}
	}
	var replacee *Token
	for i := z.pos; i < z.pos+n; i++ {
		attr := &z.buf[i]
		for j, hash := range hashes {
			if hash == attr.Hash {
				z.attrBuffer[j] = attr
				replacee = attr
			}
		}
	}
	return z.attrBuffer, replacee
}
