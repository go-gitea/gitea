package lexer

import (
	"bytes"
	"unicode/utf8"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// Lexer turns graphql request and schema strings into tokens
type Lexer struct {
	*ast.Source
	// An offset into the string in bytes
	start int
	// An offset into the string in runes
	startRunes int
	// An offset into the string in bytes
	end int
	// An offset into the string in runes
	endRunes int
	// the current line number
	line int
	// An offset into the string in rune
	lineStartRunes int
}

func New(src *ast.Source) Lexer {
	return Lexer{
		Source: src,
		line:   1,
	}
}

// take one rune from input and advance end
func (s *Lexer) peek() (rune, int) {
	return utf8.DecodeRuneInString(s.Input[s.end:])
}

func (s *Lexer) makeToken(kind Type) (Token, *gqlerror.Error) {
	return s.makeValueToken(kind, s.Input[s.start:s.end])
}

func (s *Lexer) makeValueToken(kind Type, value string) (Token, *gqlerror.Error) {
	return Token{
		Kind:  kind,
		Value: value,
		Pos: ast.Position{
			Start:  s.startRunes,
			End:    s.endRunes,
			Line:   s.line,
			Column: s.startRunes - s.lineStartRunes + 1,
			Src:    s.Source,
		},
	}, nil
}

func (s *Lexer) makeError(format string, args ...interface{}) (Token, *gqlerror.Error) {
	column := s.endRunes - s.lineStartRunes + 1
	return Token{
		Kind: Invalid,
		Pos: ast.Position{
			Start:  s.startRunes,
			End:    s.endRunes,
			Line:   s.line,
			Column: column,
			Src:    s.Source,
		},
	}, gqlerror.ErrorLocf(s.Source.Name, s.line, column, format, args...)
}

// ReadToken gets the next token from the source starting at the given position.
//
// This skips over whitespace and comments until it finds the next lexable
// token, then lexes punctuators immediately or calls the appropriate helper
// function for more complicated tokens.
func (s *Lexer) ReadToken() (token Token, err *gqlerror.Error) {

	s.ws()
	s.start = s.end
	s.startRunes = s.endRunes

	if s.end >= len(s.Input) {
		return s.makeToken(EOF)
	}
	r := s.Input[s.start]
	s.end++
	s.endRunes++
	switch r {
	case '!':
		return s.makeValueToken(Bang, "")

	case '$':
		return s.makeValueToken(Dollar, "")
	case '&':
		return s.makeValueToken(Amp, "")
	case '(':
		return s.makeValueToken(ParenL, "")
	case ')':
		return s.makeValueToken(ParenR, "")
	case '.':
		if len(s.Input) > s.start+2 && s.Input[s.start:s.start+3] == "..." {
			s.end += 2
			s.endRunes += 2
			return s.makeValueToken(Spread, "")
		}
	case ':':
		return s.makeValueToken(Colon, "")
	case '=':
		return s.makeValueToken(Equals, "")
	case '@':
		return s.makeValueToken(At, "")
	case '[':
		return s.makeValueToken(BracketL, "")
	case ']':
		return s.makeValueToken(BracketR, "")
	case '{':
		return s.makeValueToken(BraceL, "")
	case '}':
		return s.makeValueToken(BraceR, "")
	case '|':
		return s.makeValueToken(Pipe, "")
	case '#':
		s.readComment()
		return s.ReadToken()

	case '_', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z':
		return s.readName()

	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return s.readNumber()

	case '"':
		if len(s.Input) > s.start+2 && s.Input[s.start:s.start+3] == `"""` {
			return s.readBlockString()
		}

		return s.readString()
	}

	s.end--
	s.endRunes--

	if r < 0x0020 && r != 0x0009 && r != 0x000a && r != 0x000d {
		return s.makeError(`Cannot contain the invalid character "\u%04d"`, r)
	}

	if r == '\'' {
		return s.makeError(`Unexpected single quote character ('), did you mean to use a double quote (")?`)
	}

	return s.makeError(`Cannot parse the unexpected character "%s".`, string(r))
}

// ws reads from body starting at startPosition until it finds a non-whitespace
// or commented character, and updates the token end to include all whitespace
func (s *Lexer) ws() {
	for s.end < len(s.Input) {
		switch s.Input[s.end] {
		case '\t', ' ', ',':
			s.end++
			s.endRunes++
		case '\n':
			s.end++
			s.endRunes++
			s.line++
			s.lineStartRunes = s.endRunes
		case '\r':
			s.end++
			s.endRunes++
			s.line++
			s.lineStartRunes = s.endRunes
			// skip the following newline if its there
			if s.end < len(s.Input) && s.Input[s.end] == '\n' {
				s.end++
				s.endRunes++
			}
			// byte order mark, given ws is hot path we aren't relying on the unicode package here.
		case 0xef:
			if s.end+2 < len(s.Input) && s.Input[s.end+1] == 0xBB && s.Input[s.end+2] == 0xBF {
				s.end += 3
				s.endRunes++
			} else {
				return
			}
		default:
			return
		}
	}
}

// readComment from the input
//
// #[\u0009\u0020-\uFFFF]*
func (s *Lexer) readComment() (Token, *gqlerror.Error) {
	for s.end < len(s.Input) {
		r, w := s.peek()

		// SourceCharacter but not LineTerminator
		if r > 0x001f || r == '\t' {
			s.end += w
			s.endRunes++
		} else {
			break
		}
	}

	return s.makeToken(Comment)
}

// readNumber from the input, either a float
// or an int depending on whether a decimal point appears.
//
// Int:   -?(0|[1-9][0-9]*)
// Float: -?(0|[1-9][0-9]*)(\.[0-9]+)?((E|e)(+|-)?[0-9]+)?
func (s *Lexer) readNumber() (Token, *gqlerror.Error) {
	float := false

	// backup to the first digit
	s.end--
	s.endRunes--

	s.acceptByte('-')

	if s.acceptByte('0') {
		if consumed := s.acceptDigits(); consumed != 0 {
			s.end -= consumed
			s.endRunes -= consumed
			return s.makeError("Invalid number, unexpected digit after 0: %s.", s.describeNext())
		}
	} else {
		if consumed := s.acceptDigits(); consumed == 0 {
			return s.makeError("Invalid number, expected digit but got: %s.", s.describeNext())
		}
	}

	if s.acceptByte('.') {
		float = true

		if consumed := s.acceptDigits(); consumed == 0 {
			return s.makeError("Invalid number, expected digit but got: %s.", s.describeNext())
		}
	}

	if s.acceptByte('e', 'E') {
		float = true

		s.acceptByte('-', '+')

		if consumed := s.acceptDigits(); consumed == 0 {
			return s.makeError("Invalid number, expected digit but got: %s.", s.describeNext())
		}
	}

	if float {
		return s.makeToken(Float)
	} else {
		return s.makeToken(Int)
	}
}

// acceptByte if it matches any of given bytes, returning true if it found anything
func (s *Lexer) acceptByte(bytes ...uint8) bool {
	if s.end >= len(s.Input) {
		return false
	}

	for _, accepted := range bytes {
		if s.Input[s.end] == accepted {
			s.end++
			s.endRunes++
			return true
		}
	}
	return false
}

// acceptDigits from the input, returning the number of digits it found
func (s *Lexer) acceptDigits() int {
	consumed := 0
	for s.end < len(s.Input) && s.Input[s.end] >= '0' && s.Input[s.end] <= '9' {
		s.end++
		s.endRunes++
		consumed++
	}

	return consumed
}

// describeNext peeks at the input and returns a human readable string. This should will alloc
// and should only be used in errors
func (s *Lexer) describeNext() string {
	if s.end < len(s.Input) {
		return `"` + string(s.Input[s.end]) + `"`
	}
	return "<EOF>"
}

// readString from the input
//
// "([^"\\\u000A\u000D]|(\\(u[0-9a-fA-F]{4}|["\\/bfnrt])))*"
func (s *Lexer) readString() (Token, *gqlerror.Error) {
	inputLen := len(s.Input)

	// this buffer is lazily created only if there are escape characters.
	var buf *bytes.Buffer

	// skip the opening quote
	s.start++
	s.startRunes++

	for s.end < inputLen {
		r := s.Input[s.end]
		if r == '\n' || r == '\r' {
			break
		}
		if r < 0x0020 && r != '\t' {
			return s.makeError(`Invalid character within String: "\u%04d".`, r)
		}
		switch r {
		default:
			var char = rune(r)
			var w = 1

			// skip unicode overhead if we are in the ascii range
			if r >= 127 {
				char, w = utf8.DecodeRuneInString(s.Input[s.end:])
			}
			s.end += w
			s.endRunes++

			if buf != nil {
				buf.WriteRune(char)
			}

		case '"':
			t, err := s.makeToken(String)
			// the token should not include the quotes in its value, but should cover them in its position
			t.Pos.Start--
			t.Pos.End++

			if buf != nil {
				t.Value = buf.String()
			}

			// skip the close quote
			s.end++
			s.endRunes++

			return t, err

		case '\\':
			if s.end+1 >= inputLen {
				s.end++
				s.endRunes++
				return s.makeError(`Invalid character escape sequence.`)
			}

			if buf == nil {
				buf = bytes.NewBufferString(s.Input[s.start:s.end])
			}

			escape := s.Input[s.end+1]

			if escape == 'u' {
				if s.end+6 >= inputLen {
					s.end++
					s.endRunes++
					return s.makeError("Invalid character escape sequence: \\%s.", s.Input[s.end:])
				}

				r, ok := unhex(s.Input[s.end+2 : s.end+6])
				if !ok {
					s.end++
					s.endRunes++
					return s.makeError("Invalid character escape sequence: \\%s.", s.Input[s.end:s.end+5])
				}
				buf.WriteRune(r)
				s.end += 6
				s.endRunes += 6
			} else {
				switch escape {
				case '"', '/', '\\':
					buf.WriteByte(escape)
				case 'b':
					buf.WriteByte('\b')
				case 'f':
					buf.WriteByte('\f')
				case 'n':
					buf.WriteByte('\n')
				case 'r':
					buf.WriteByte('\r')
				case 't':
					buf.WriteByte('\t')
				default:
					s.end += 1
					s.endRunes += 1
					return s.makeError("Invalid character escape sequence: \\%s.", string(escape))
				}
				s.end += 2
				s.endRunes += 2
			}
		}
	}

	return s.makeError("Unterminated string.")
}

// readBlockString from the input
//
// """("?"?(\\"""|\\(?!=""")|[^"\\]))*"""
func (s *Lexer) readBlockString() (Token, *gqlerror.Error) {
	inputLen := len(s.Input)

	var buf bytes.Buffer

	// skip the opening quote
	s.start += 3
	s.startRunes += 3
	s.end += 2
	s.endRunes += 2

	for s.end < inputLen {
		r := s.Input[s.end]

		// Closing triple quote (""")
		if r == '"' && s.end+3 <= inputLen && s.Input[s.end:s.end+3] == `"""` {
			t, err := s.makeValueToken(BlockString, blockStringValue(buf.String()))

			// the token should not include the quotes in its value, but should cover them in its position
			t.Pos.Start -= 3
			t.Pos.End += 3

			// skip the close quote
			s.end += 3
			s.endRunes += 3

			return t, err
		}

		// SourceCharacter
		if r < 0x0020 && r != '\t' && r != '\n' && r != '\r' {
			return s.makeError(`Invalid character within String: "\u%04d".`, r)
		}

		if r == '\\' && s.end+4 <= inputLen && s.Input[s.end:s.end+4] == `\"""` {
			buf.WriteString(`"""`)
			s.end += 4
			s.endRunes += 4
		} else if r == '\r' {
			if s.end+1 < inputLen && s.Input[s.end+1] == '\n' {
				s.end++
				s.endRunes++
			}

			buf.WriteByte('\n')
			s.end++
			s.endRunes++
		} else {
			var char = rune(r)
			var w = 1

			// skip unicode overhead if we are in the ascii range
			if r >= 127 {
				char, w = utf8.DecodeRuneInString(s.Input[s.end:])
			}
			s.end += w
			s.endRunes++
			buf.WriteRune(char)
		}
	}

	return s.makeError("Unterminated string.")
}

func unhex(b string) (v rune, ok bool) {
	for _, c := range b {
		v <<= 4
		switch {
		case '0' <= c && c <= '9':
			v |= c - '0'
		case 'a' <= c && c <= 'f':
			v |= c - 'a' + 10
		case 'A' <= c && c <= 'F':
			v |= c - 'A' + 10
		default:
			return 0, false
		}
	}

	return v, true
}

// readName from the input
//
// [_A-Za-z][_0-9A-Za-z]*
func (s *Lexer) readName() (Token, *gqlerror.Error) {
	for s.end < len(s.Input) {
		r, w := s.peek()

		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_' {
			s.end += w
			s.endRunes++
		} else {
			break
		}
	}

	return s.makeToken(Name)
}
