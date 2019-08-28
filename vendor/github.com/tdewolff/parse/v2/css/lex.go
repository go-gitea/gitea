// Package css is a CSS3 lexer and parser following the specifications at http://www.w3.org/TR/css-syntax-3/.
package css

// TODO: \uFFFD replacement character for NULL bytes in strings for example, or atleast don't end the string early

import (
	"bytes"
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
	IdentToken
	FunctionToken  // rgb( rgba( ...
	AtKeywordToken // @abc
	HashToken      // #abc
	StringToken
	BadStringToken
	URLToken
	BadURLToken
	DelimToken            // any unmatched character
	NumberToken           // 5
	PercentageToken       // 5%
	DimensionToken        // 5em
	UnicodeRangeToken     // U+554A
	IncludeMatchToken     // ~=
	DashMatchToken        // |=
	PrefixMatchToken      // ^=
	SuffixMatchToken      // $=
	SubstringMatchToken   // *=
	ColumnToken           // ||
	WhitespaceToken       // space \t \r \n \f
	CDOToken              // <!--
	CDCToken              // -->
	ColonToken            // :
	SemicolonToken        // ;
	CommaToken            // ,
	LeftBracketToken      // [
	RightBracketToken     // ]
	LeftParenthesisToken  // (
	RightParenthesisToken // )
	LeftBraceToken        // {
	RightBraceToken       // }
	CommentToken          // extra token for comments
	EmptyToken
	CustomPropertyNameToken
	CustomPropertyValueToken
)

// String returns the string representation of a TokenType.
func (tt TokenType) String() string {
	switch tt {
	case ErrorToken:
		return "Error"
	case IdentToken:
		return "Ident"
	case FunctionToken:
		return "Function"
	case AtKeywordToken:
		return "AtKeyword"
	case HashToken:
		return "Hash"
	case StringToken:
		return "String"
	case BadStringToken:
		return "BadString"
	case URLToken:
		return "URL"
	case BadURLToken:
		return "BadURL"
	case DelimToken:
		return "Delim"
	case NumberToken:
		return "Number"
	case PercentageToken:
		return "Percentage"
	case DimensionToken:
		return "Dimension"
	case UnicodeRangeToken:
		return "UnicodeRange"
	case IncludeMatchToken:
		return "IncludeMatch"
	case DashMatchToken:
		return "DashMatch"
	case PrefixMatchToken:
		return "PrefixMatch"
	case SuffixMatchToken:
		return "SuffixMatch"
	case SubstringMatchToken:
		return "SubstringMatch"
	case ColumnToken:
		return "Column"
	case WhitespaceToken:
		return "Whitespace"
	case CDOToken:
		return "CDO"
	case CDCToken:
		return "CDC"
	case ColonToken:
		return "Colon"
	case SemicolonToken:
		return "Semicolon"
	case CommaToken:
		return "Comma"
	case LeftBracketToken:
		return "LeftBracket"
	case RightBracketToken:
		return "RightBracket"
	case LeftParenthesisToken:
		return "LeftParenthesis"
	case RightParenthesisToken:
		return "RightParenthesis"
	case LeftBraceToken:
		return "LeftBrace"
	case RightBraceToken:
		return "RightBrace"
	case CommentToken:
		return "Comment"
	case EmptyToken:
		return "Empty"
	case CustomPropertyNameToken:
		return "CustomPropertyName"
	case CustomPropertyValueToken:
		return "CustomPropertyValue"
	}
	return "Invalid(" + strconv.Itoa(int(tt)) + ")"
}

////////////////////////////////////////////////////////////////

// Lexer is the state for the lexer.
type Lexer struct {
	r *buffer.Lexer
}

// NewLexer returns a new Lexer for a given io.Reader.
func NewLexer(r io.Reader) *Lexer {
	return &Lexer{
		buffer.NewLexer(r),
	}
}

// Err returns the error encountered during lexing, this is often io.EOF but also other errors can be returned.
func (l *Lexer) Err() error {
	return l.r.Err()
}

// Restore restores the NULL byte at the end of the buffer.
func (l *Lexer) Restore() {
	l.r.Restore()
}

// Next returns the next Token. It returns ErrorToken when an error was encountered. Using Err() one can retrieve the error message.
func (l *Lexer) Next() (TokenType, []byte) {
	switch l.r.Peek(0) {
	case ' ', '\t', '\n', '\r', '\f':
		l.r.Move(1)
		for l.consumeWhitespace() {
		}
		return WhitespaceToken, l.r.Shift()
	case ':':
		l.r.Move(1)
		return ColonToken, l.r.Shift()
	case ';':
		l.r.Move(1)
		return SemicolonToken, l.r.Shift()
	case ',':
		l.r.Move(1)
		return CommaToken, l.r.Shift()
	case '(', ')', '[', ']', '{', '}':
		if t := l.consumeBracket(); t != ErrorToken {
			return t, l.r.Shift()
		}
	case '#':
		if l.consumeHashToken() {
			return HashToken, l.r.Shift()
		}
	case '"', '\'':
		if t := l.consumeString(); t != ErrorToken {
			return t, l.r.Shift()
		}
	case '.', '+':
		if t := l.consumeNumeric(); t != ErrorToken {
			return t, l.r.Shift()
		}
	case '-':
		if t := l.consumeNumeric(); t != ErrorToken {
			return t, l.r.Shift()
		} else if t := l.consumeIdentlike(); t != ErrorToken {
			return t, l.r.Shift()
		} else if l.consumeCDCToken() {
			return CDCToken, l.r.Shift()
		} else if l.consumeCustomVariableToken() {
			return CustomPropertyNameToken, l.r.Shift()
		}
	case '@':
		if l.consumeAtKeywordToken() {
			return AtKeywordToken, l.r.Shift()
		}
	case '$', '*', '^', '~':
		if t := l.consumeMatch(); t != ErrorToken {
			return t, l.r.Shift()
		}
	case '/':
		if l.consumeComment() {
			return CommentToken, l.r.Shift()
		}
	case '<':
		if l.consumeCDOToken() {
			return CDOToken, l.r.Shift()
		}
	case '\\':
		if t := l.consumeIdentlike(); t != ErrorToken {
			return t, l.r.Shift()
		}
	case 'u', 'U':
		if l.consumeUnicodeRangeToken() {
			return UnicodeRangeToken, l.r.Shift()
		} else if t := l.consumeIdentlike(); t != ErrorToken {
			return t, l.r.Shift()
		}
	case '|':
		if t := l.consumeMatch(); t != ErrorToken {
			return t, l.r.Shift()
		} else if l.consumeColumnToken() {
			return ColumnToken, l.r.Shift()
		}
	case 0:
		if l.r.Err() != nil {
			return ErrorToken, nil
		}
	default:
		if t := l.consumeNumeric(); t != ErrorToken {
			return t, l.r.Shift()
		} else if t := l.consumeIdentlike(); t != ErrorToken {
			return t, l.r.Shift()
		}
	}
	// can't be rune because consumeIdentlike consumes that as an identifier
	l.r.Move(1)
	return DelimToken, l.r.Shift()
}

////////////////////////////////////////////////////////////////

/*
The following functions follow the railroad diagrams in http://www.w3.org/TR/css3-syntax/
*/

func (l *Lexer) consumeByte(c byte) bool {
	if l.r.Peek(0) == c {
		l.r.Move(1)
		return true
	}
	return false
}

func (l *Lexer) consumeComment() bool {
	if l.r.Peek(0) != '/' || l.r.Peek(1) != '*' {
		return false
	}
	l.r.Move(2)
	for {
		c := l.r.Peek(0)
		if c == 0 && l.r.Err() != nil {
			break
		} else if c == '*' && l.r.Peek(1) == '/' {
			l.r.Move(2)
			return true
		}
		l.r.Move(1)
	}
	return true
}

func (l *Lexer) consumeNewline() bool {
	c := l.r.Peek(0)
	if c == '\n' || c == '\f' {
		l.r.Move(1)
		return true
	} else if c == '\r' {
		if l.r.Peek(1) == '\n' {
			l.r.Move(2)
		} else {
			l.r.Move(1)
		}
		return true
	}
	return false
}

func (l *Lexer) consumeWhitespace() bool {
	c := l.r.Peek(0)
	if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' {
		l.r.Move(1)
		return true
	}
	return false
}

func (l *Lexer) consumeDigit() bool {
	c := l.r.Peek(0)
	if c >= '0' && c <= '9' {
		l.r.Move(1)
		return true
	}
	return false
}

func (l *Lexer) consumeHexDigit() bool {
	c := l.r.Peek(0)
	if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
		l.r.Move(1)
		return true
	}
	return false
}

func (l *Lexer) consumeEscape() bool {
	if l.r.Peek(0) != '\\' {
		return false
	}
	mark := l.r.Pos()
	l.r.Move(1)
	if l.consumeNewline() {
		l.r.Rewind(mark)
		return false
	} else if l.consumeHexDigit() {
		for k := 1; k < 6; k++ {
			if !l.consumeHexDigit() {
				break
			}
		}
		l.consumeWhitespace()
		return true
	} else {
		c := l.r.Peek(0)
		if c >= 0xC0 {
			_, n := l.r.PeekRune(0)
			l.r.Move(n)
			return true
		} else if c == 0 && l.r.Err() != nil {
			return true
		}
	}
	l.r.Move(1)
	return true
}

func (l *Lexer) consumeIdentToken() bool {
	mark := l.r.Pos()
	if l.r.Peek(0) == '-' {
		l.r.Move(1)
	}
	c := l.r.Peek(0)
	if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c >= 0x80) {
		if c != '\\' || !l.consumeEscape() {
			l.r.Rewind(mark)
			return false
		}
	} else {
		l.r.Move(1)
	}
	for {
		c := l.r.Peek(0)
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c >= 0x80) {
			if c != '\\' || !l.consumeEscape() {
				break
			}
		} else {
			l.r.Move(1)
		}
	}
	return true
}

// support custom variables, https://www.w3.org/TR/css-variables-1/
func (l *Lexer) consumeCustomVariableToken() bool {
	// expect to be on a '-'
	l.r.Move(1)
	if l.r.Peek(0) != '-' {
		l.r.Move(-1)
		return false
	}
	if !l.consumeIdentToken() {
		l.r.Move(-1)
		return false
	}
	return true
}

func (l *Lexer) consumeAtKeywordToken() bool {
	// expect to be on an '@'
	l.r.Move(1)
	if !l.consumeIdentToken() {
		l.r.Move(-1)
		return false
	}
	return true
}

func (l *Lexer) consumeHashToken() bool {
	// expect to be on a '#'
	mark := l.r.Pos()
	l.r.Move(1)
	c := l.r.Peek(0)
	if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c >= 0x80) {
		if c != '\\' || !l.consumeEscape() {
			l.r.Rewind(mark)
			return false
		}
	} else {
		l.r.Move(1)
	}
	for {
		c := l.r.Peek(0)
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c >= 0x80) {
			if c != '\\' || !l.consumeEscape() {
				break
			}
		} else {
			l.r.Move(1)
		}
	}
	return true
}

func (l *Lexer) consumeNumberToken() bool {
	mark := l.r.Pos()
	c := l.r.Peek(0)
	if c == '+' || c == '-' {
		l.r.Move(1)
	}
	firstDigit := l.consumeDigit()
	if firstDigit {
		for l.consumeDigit() {
		}
	}
	if l.r.Peek(0) == '.' {
		l.r.Move(1)
		if l.consumeDigit() {
			for l.consumeDigit() {
			}
		} else if firstDigit {
			// . could belong to the next token
			l.r.Move(-1)
			return true
		} else {
			l.r.Rewind(mark)
			return false
		}
	} else if !firstDigit {
		l.r.Rewind(mark)
		return false
	}
	mark = l.r.Pos()
	c = l.r.Peek(0)
	if c == 'e' || c == 'E' {
		l.r.Move(1)
		c = l.r.Peek(0)
		if c == '+' || c == '-' {
			l.r.Move(1)
		}
		if !l.consumeDigit() {
			// e could belong to next token
			l.r.Rewind(mark)
			return true
		}
		for l.consumeDigit() {
		}
	}
	return true
}

func (l *Lexer) consumeUnicodeRangeToken() bool {
	c := l.r.Peek(0)
	if (c != 'u' && c != 'U') || l.r.Peek(1) != '+' {
		return false
	}
	mark := l.r.Pos()
	l.r.Move(2)
	if l.consumeHexDigit() {
		// consume up to 6 hexDigits
		k := 1
		for ; k < 6; k++ {
			if !l.consumeHexDigit() {
				break
			}
		}

		// either a minus or a question mark or the end is expected
		if l.consumeByte('-') {
			// consume another up to 6 hexDigits
			if l.consumeHexDigit() {
				for k := 1; k < 6; k++ {
					if !l.consumeHexDigit() {
						break
					}
				}
			} else {
				l.r.Rewind(mark)
				return false
			}
		} else {
			// could be filled up to 6 characters with question marks or else regular hexDigits
			if l.consumeByte('?') {
				k++
				for ; k < 6; k++ {
					if !l.consumeByte('?') {
						l.r.Rewind(mark)
						return false
					}
				}
			}
		}
	} else {
		// consume 6 question marks
		for k := 0; k < 6; k++ {
			if !l.consumeByte('?') {
				l.r.Rewind(mark)
				return false
			}
		}
	}
	return true
}

func (l *Lexer) consumeColumnToken() bool {
	if l.r.Peek(0) == '|' && l.r.Peek(1) == '|' {
		l.r.Move(2)
		return true
	}
	return false
}

func (l *Lexer) consumeCDOToken() bool {
	if l.r.Peek(0) == '<' && l.r.Peek(1) == '!' && l.r.Peek(2) == '-' && l.r.Peek(3) == '-' {
		l.r.Move(4)
		return true
	}
	return false
}

func (l *Lexer) consumeCDCToken() bool {
	if l.r.Peek(0) == '-' && l.r.Peek(1) == '-' && l.r.Peek(2) == '>' {
		l.r.Move(3)
		return true
	}
	return false
}

////////////////////////////////////////////////////////////////

// consumeMatch consumes any MatchToken.
func (l *Lexer) consumeMatch() TokenType {
	if l.r.Peek(1) == '=' {
		switch l.r.Peek(0) {
		case '~':
			l.r.Move(2)
			return IncludeMatchToken
		case '|':
			l.r.Move(2)
			return DashMatchToken
		case '^':
			l.r.Move(2)
			return PrefixMatchToken
		case '$':
			l.r.Move(2)
			return SuffixMatchToken
		case '*':
			l.r.Move(2)
			return SubstringMatchToken
		}
	}
	return ErrorToken
}

// consumeBracket consumes any bracket token.
func (l *Lexer) consumeBracket() TokenType {
	switch l.r.Peek(0) {
	case '(':
		l.r.Move(1)
		return LeftParenthesisToken
	case ')':
		l.r.Move(1)
		return RightParenthesisToken
	case '[':
		l.r.Move(1)
		return LeftBracketToken
	case ']':
		l.r.Move(1)
		return RightBracketToken
	case '{':
		l.r.Move(1)
		return LeftBraceToken
	case '}':
		l.r.Move(1)
		return RightBraceToken
	}
	return ErrorToken
}

// consumeNumeric consumes NumberToken, PercentageToken or DimensionToken.
func (l *Lexer) consumeNumeric() TokenType {
	if l.consumeNumberToken() {
		if l.consumeByte('%') {
			return PercentageToken
		} else if l.consumeIdentToken() {
			return DimensionToken
		}
		return NumberToken
	}
	return ErrorToken
}

// consumeString consumes a string and may return BadStringToken when a newline is encountered.
func (l *Lexer) consumeString() TokenType {
	// assume to be on " or '
	delim := l.r.Peek(0)
	l.r.Move(1)
	for {
		c := l.r.Peek(0)
		if c == 0 && l.r.Err() != nil {
			break
		} else if c == '\n' || c == '\r' || c == '\f' {
			l.r.Move(1)
			return BadStringToken
		} else if c == delim {
			l.r.Move(1)
			break
		} else if c == '\\' {
			if !l.consumeEscape() {
				l.r.Move(1)
				l.consumeNewline()
			}
		} else {
			l.r.Move(1)
		}
	}
	return StringToken
}

func (l *Lexer) consumeUnquotedURL() bool {
	for {
		c := l.r.Peek(0)
		if c == 0 && l.r.Err() != nil || c == ')' {
			break
		} else if c == '"' || c == '\'' || c == '(' || c == '\\' || c == ' ' || c <= 0x1F || c == 0x7F {
			if c != '\\' || !l.consumeEscape() {
				return false
			}
		} else {
			l.r.Move(1)
		}
	}
	return true
}

// consumeRemnantsBadUrl consumes bytes of a BadUrlToken so that normal tokenization may continue.
func (l *Lexer) consumeRemnantsBadURL() {
	for {
		if l.consumeByte(')') || l.r.Err() != nil {
			break
		} else if !l.consumeEscape() {
			l.r.Move(1)
		}
	}
}

// consumeIdentlike consumes IdentToken, FunctionToken or UrlToken.
func (l *Lexer) consumeIdentlike() TokenType {
	if l.consumeIdentToken() {
		if l.r.Peek(0) != '(' {
			return IdentToken
		} else if !parse.EqualFold(bytes.Replace(l.r.Lexeme(), []byte{'\\'}, nil, -1), []byte{'u', 'r', 'l'}) {
			l.r.Move(1)
			return FunctionToken
		}
		l.r.Move(1)

		// consume url
		for l.consumeWhitespace() {
		}
		if c := l.r.Peek(0); c == '"' || c == '\'' {
			if l.consumeString() == BadStringToken {
				l.consumeRemnantsBadURL()
				return BadURLToken
			}
		} else if !l.consumeUnquotedURL() && !l.consumeWhitespace() { // if unquoted URL fails due to encountering whitespace, continue
			l.consumeRemnantsBadURL()
			return BadURLToken
		}
		for l.consumeWhitespace() {
		}
		if !l.consumeByte(')') && l.r.Err() != io.EOF {
			l.consumeRemnantsBadURL()
			return BadURLToken
		}
		return URLToken
	}
	return ErrorToken
}
