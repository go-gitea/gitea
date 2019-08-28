// Package xml is an XML1.0 lexer following the specifications at http://www.w3.org/TR/xml/.
package xml

import (
	"io"
	"strconv"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/buffer"
)

// TokenType determines the type of token, eg. a number or a semicolon.
type TokenType uint32

// TokenType values.
const (
	ErrorToken TokenType = iota // extra token when errors occur
	CommentToken
	DOCTYPEToken
	CDATAToken
	StartTagToken
	StartTagPIToken
	StartTagCloseToken
	StartTagCloseVoidToken
	StartTagClosePIToken
	EndTagToken
	AttributeToken
	TextToken
)

// String returns the string representation of a TokenType.
func (tt TokenType) String() string {
	switch tt {
	case ErrorToken:
		return "Error"
	case CommentToken:
		return "Comment"
	case DOCTYPEToken:
		return "DOCTYPE"
	case CDATAToken:
		return "CDATA"
	case StartTagToken:
		return "StartTag"
	case StartTagPIToken:
		return "StartTagPI"
	case StartTagCloseToken:
		return "StartTagClose"
	case StartTagCloseVoidToken:
		return "StartTagCloseVoid"
	case StartTagClosePIToken:
		return "StartTagClosePI"
	case EndTagToken:
		return "EndTag"
	case AttributeToken:
		return "Attribute"
	case TextToken:
		return "Text"
	}
	return "Invalid(" + strconv.Itoa(int(tt)) + ")"
}

////////////////////////////////////////////////////////////////

// Lexer is the state for the lexer.
type Lexer struct {
	r   *buffer.Lexer
	err error

	inTag bool

	text    []byte
	attrVal []byte
}

// NewLexer returns a new Lexer for a given io.Reader.
func NewLexer(r io.Reader) *Lexer {
	return &Lexer{
		r: buffer.NewLexer(r),
	}
}

// Err returns the error encountered during lexing, this is often io.EOF but also other errors can be returned.
func (l *Lexer) Err() error {
	if l.err != nil {
		return l.err
	}
	return l.r.Err()
}

// Restore restores the NULL byte at the end of the buffer.
func (l *Lexer) Restore() {
	l.r.Restore()
}

// Next returns the next Token. It returns ErrorToken when an error was encountered. Using Err() one can retrieve the error message.
func (l *Lexer) Next() (TokenType, []byte) {
	l.text = nil
	var c byte
	if l.inTag {
		l.attrVal = nil
		for { // before attribute name state
			if c = l.r.Peek(0); c == ' ' || c == '\t' || c == '\n' || c == '\r' {
				l.r.Move(1)
				continue
			}
			break
		}
		if c == 0 {
			if l.r.Err() == nil {
				l.err = parse.NewErrorLexer("unexpected null character", l.r)
			}
			return ErrorToken, nil
		} else if c != '>' && (c != '/' && c != '?' || l.r.Peek(1) != '>') {
			return AttributeToken, l.shiftAttribute()
		}
		start := l.r.Pos()
		l.inTag = false
		if c == '/' {
			l.r.Move(2)
			l.text = l.r.Lexeme()[start:]
			return StartTagCloseVoidToken, l.r.Shift()
		} else if c == '?' {
			l.r.Move(2)
			l.text = l.r.Lexeme()[start:]
			return StartTagClosePIToken, l.r.Shift()
		} else {
			l.r.Move(1)
			l.text = l.r.Lexeme()[start:]
			return StartTagCloseToken, l.r.Shift()
		}
	}

	for {
		c = l.r.Peek(0)
		if c == '<' {
			if l.r.Pos() > 0 {
				return TextToken, l.r.Shift()
			}
			c = l.r.Peek(1)
			if c == '/' {
				l.r.Move(2)
				return EndTagToken, l.shiftEndTag()
			} else if c == '!' {
				l.r.Move(2)
				if l.at('-', '-') {
					l.r.Move(2)
					return CommentToken, l.shiftCommentText()
				} else if l.at('[', 'C', 'D', 'A', 'T', 'A', '[') {
					l.r.Move(7)
					return CDATAToken, l.shiftCDATAText()
				} else if l.at('D', 'O', 'C', 'T', 'Y', 'P', 'E') {
					l.r.Move(7)
					return DOCTYPEToken, l.shiftDOCTYPEText()
				}
				l.r.Move(-2)
			} else if c == '?' {
				l.r.Move(2)
				l.inTag = true
				return StartTagPIToken, l.shiftStartTag()
			}
			l.r.Move(1)
			l.inTag = true
			return StartTagToken, l.shiftStartTag()
		} else if c == 0 {
			if l.r.Pos() > 0 {
				return TextToken, l.r.Shift()
			}
			if l.r.Err() == nil {
				l.err = parse.NewErrorLexer("unexpected null character", l.r)
			}
			return ErrorToken, nil
		}
		l.r.Move(1)
	}
}

// Text returns the textual representation of a token. This excludes delimiters and additional leading/trailing characters.
func (l *Lexer) Text() []byte {
	return l.text
}

// AttrVal returns the attribute value when an AttributeToken was returned from Next.
func (l *Lexer) AttrVal() []byte {
	return l.attrVal
}

////////////////////////////////////////////////////////////////

// The following functions follow the specifications at http://www.w3.org/html/wg/drafts/html/master/syntax.html

func (l *Lexer) shiftDOCTYPEText() []byte {
	inString := false
	inBrackets := false
	for {
		c := l.r.Peek(0)
		if c == '"' {
			inString = !inString
		} else if (c == '[' || c == ']') && !inString {
			inBrackets = (c == '[')
		} else if c == '>' && !inString && !inBrackets {
			l.text = l.r.Lexeme()[9:]
			l.r.Move(1)
			return l.r.Shift()
		} else if c == 0 {
			l.text = l.r.Lexeme()[9:]
			return l.r.Shift()
		}
		l.r.Move(1)
	}
}

func (l *Lexer) shiftCDATAText() []byte {
	for {
		c := l.r.Peek(0)
		if c == ']' && l.r.Peek(1) == ']' && l.r.Peek(2) == '>' {
			l.text = l.r.Lexeme()[9:]
			l.r.Move(3)
			return l.r.Shift()
		} else if c == 0 {
			l.text = l.r.Lexeme()[9:]
			return l.r.Shift()
		}
		l.r.Move(1)
	}
}

func (l *Lexer) shiftCommentText() []byte {
	for {
		c := l.r.Peek(0)
		if c == '-' && l.r.Peek(1) == '-' && l.r.Peek(2) == '>' {
			l.text = l.r.Lexeme()[4:]
			l.r.Move(3)
			return l.r.Shift()
		} else if c == 0 {
			return l.r.Shift()
		}
		l.r.Move(1)
	}
}

func (l *Lexer) shiftStartTag() []byte {
	nameStart := l.r.Pos()
	for {
		if c := l.r.Peek(0); c == ' ' || c == '>' || (c == '/' || c == '?') && l.r.Peek(1) == '>' || c == '\t' || c == '\n' || c == '\r' || c == 0 {
			break
		}
		l.r.Move(1)
	}
	l.text = l.r.Lexeme()[nameStart:]
	return l.r.Shift()
}

func (l *Lexer) shiftAttribute() []byte {
	nameStart := l.r.Pos()
	var c byte
	for { // attribute name state
		if c = l.r.Peek(0); c == ' ' || c == '=' || c == '>' || (c == '/' || c == '?') && l.r.Peek(1) == '>' || c == '\t' || c == '\n' || c == '\r' || c == 0 {
			break
		}
		l.r.Move(1)
	}
	nameEnd := l.r.Pos()
	for { // after attribute name state
		if c = l.r.Peek(0); c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			l.r.Move(1)
			continue
		}
		break
	}
	if c == '=' {
		l.r.Move(1)
		for { // before attribute value state
			if c = l.r.Peek(0); c == ' ' || c == '\t' || c == '\n' || c == '\r' {
				l.r.Move(1)
				continue
			}
			break
		}
		attrPos := l.r.Pos()
		delim := c
		if delim == '"' || delim == '\'' { // attribute value single- and double-quoted state
			l.r.Move(1)
			for {
				c = l.r.Peek(0)
				if c == delim {
					l.r.Move(1)
					break
				} else if c == 0 {
					break
				}
				l.r.Move(1)
				if c == '\t' || c == '\n' || c == '\r' {
					l.r.Lexeme()[l.r.Pos()-1] = ' '
				}
			}
		} else { // attribute value unquoted state
			for {
				if c = l.r.Peek(0); c == ' ' || c == '>' || (c == '/' || c == '?') && l.r.Peek(1) == '>' || c == '\t' || c == '\n' || c == '\r' || c == 0 {
					break
				}
				l.r.Move(1)
			}
		}
		l.attrVal = l.r.Lexeme()[attrPos:]
	} else {
		l.r.Rewind(nameEnd)
		l.attrVal = nil
	}
	l.text = l.r.Lexeme()[nameStart:nameEnd]
	return l.r.Shift()
}

func (l *Lexer) shiftEndTag() []byte {
	for {
		c := l.r.Peek(0)
		if c == '>' {
			l.text = l.r.Lexeme()[2:]
			l.r.Move(1)
			break
		} else if c == 0 {
			l.text = l.r.Lexeme()[2:]
			break
		}
		l.r.Move(1)
	}

	end := len(l.text)
	for end > 0 {
		if c := l.text[end-1]; c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			end--
			continue
		}
		break
	}
	l.text = l.text[:end]
	return l.r.Shift()
}

////////////////////////////////////////////////////////////////

func (l *Lexer) at(b ...byte) bool {
	for i, c := range b {
		if l.r.Peek(i) != c {
			return false
		}
	}
	return true
}
