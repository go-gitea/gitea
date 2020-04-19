package css

import (
	"bytes"
	"io"
	"strconv"

	"github.com/tdewolff/parse/v2"
)

var wsBytes = []byte(" ")
var endBytes = []byte("}")
var emptyBytes = []byte("")

// GrammarType determines the type of grammar.
type GrammarType uint32

// GrammarType values.
const (
	ErrorGrammar GrammarType = iota // extra token when errors occur
	CommentGrammar
	AtRuleGrammar
	BeginAtRuleGrammar
	EndAtRuleGrammar
	QualifiedRuleGrammar
	BeginRulesetGrammar
	EndRulesetGrammar
	DeclarationGrammar
	TokenGrammar
	CustomPropertyGrammar
)

// String returns the string representation of a GrammarType.
func (tt GrammarType) String() string {
	switch tt {
	case ErrorGrammar:
		return "Error"
	case CommentGrammar:
		return "Comment"
	case AtRuleGrammar:
		return "AtRule"
	case BeginAtRuleGrammar:
		return "BeginAtRule"
	case EndAtRuleGrammar:
		return "EndAtRule"
	case QualifiedRuleGrammar:
		return "QualifiedRule"
	case BeginRulesetGrammar:
		return "BeginRuleset"
	case EndRulesetGrammar:
		return "EndRuleset"
	case DeclarationGrammar:
		return "Declaration"
	case TokenGrammar:
		return "Token"
	case CustomPropertyGrammar:
		return "CustomProperty"
	}
	return "Invalid(" + strconv.Itoa(int(tt)) + ")"
}

////////////////////////////////////////////////////////////////

// State is the state function the parser currently is in.
type State func(*Parser) GrammarType

// Token is a single TokenType and its associated data.
type Token struct {
	TokenType
	Data []byte
}

func (t Token) String() string {
	return t.TokenType.String() + "('" + string(t.Data) + "')"
}

// Parser is the state for the parser.
type Parser struct {
	l     *Lexer
	state []State
	err   error

	buf   []Token
	level int

	tt          TokenType
	data        []byte
	keepWS      bool
	prevWS      bool
	prevEnd     bool
	prevComment bool
}

// NewParser returns a new CSS parser from an io.Reader. isInline specifies whether this is an inline style attribute.
func NewParser(r io.Reader, isInline bool) *Parser {
	l := NewLexer(r)
	p := &Parser{
		l:     l,
		state: make([]State, 0, 4),
	}

	if isInline {
		p.state = append(p.state, (*Parser).parseDeclarationList)
	} else {
		p.state = append(p.state, (*Parser).parseStylesheet)
	}
	return p
}

// Err returns the error encountered during parsing, this is often io.EOF but also other errors can be returned.
func (p *Parser) Err() error {
	if p.err != nil {
		return p.err
	}
	return p.l.Err()
}

// Restore restores the NULL byte at the end of the buffer.
func (p *Parser) Restore() {
	p.l.Restore()
}

// Offset returns the current position in the input stream.
func (p *Parser) Offset() int {
	return p.l.Offset()
}

// Next returns the next Grammar. It returns ErrorGrammar when an error was encountered. Using Err() one can retrieve the error message.
func (p *Parser) Next() (GrammarType, TokenType, []byte) {
	p.err = nil

	if p.prevEnd {
		p.tt, p.data = RightBraceToken, endBytes
		p.prevEnd = false
	} else {
		p.tt, p.data = p.popToken(true)
	}
	gt := p.state[len(p.state)-1](p)
	return gt, p.tt, p.data
}

// Values returns a slice of Tokens for the last Grammar. Only AtRuleGrammar, BeginAtRuleGrammar, BeginRulesetGrammar and Declaration will return the at-rule components, ruleset selector and declaration values respectively.
func (p *Parser) Values() []Token {
	return p.buf
}

func (p *Parser) popToken(allowComment bool) (TokenType, []byte) {
	p.prevWS = false
	p.prevComment = false
	tt, data := p.l.Next()
	for !p.keepWS && tt == WhitespaceToken || tt == CommentToken {
		if tt == WhitespaceToken {
			p.prevWS = true
		} else {
			p.prevComment = true
			if allowComment && len(p.state) == 1 {
				break
			}
		}
		tt, data = p.l.Next()
	}
	return tt, data
}

func (p *Parser) initBuf() {
	p.buf = p.buf[:0]
}

func (p *Parser) pushBuf(tt TokenType, data []byte) {
	p.buf = append(p.buf, Token{tt, data})
}

////////////////////////////////////////////////////////////////

func (p *Parser) parseStylesheet() GrammarType {
	if p.tt == CDOToken || p.tt == CDCToken {
		return TokenGrammar
	} else if p.tt == AtKeywordToken {
		return p.parseAtRule()
	} else if p.tt == CommentToken {
		return CommentGrammar
	} else if p.tt == ErrorToken {
		return ErrorGrammar
	}
	return p.parseQualifiedRule()
}

func (p *Parser) parseDeclarationList() GrammarType {
	if p.tt == CommentToken {
		p.tt, p.data = p.popToken(false)
	}
	for p.tt == SemicolonToken {
		p.tt, p.data = p.popToken(false)
	}

	// IE hack: *color:red;
	if p.tt == DelimToken && p.data[0] == '*' {
		tt, data := p.popToken(false)
		p.tt = tt
		p.data = append(p.data, data...)
	}

	if p.tt == ErrorToken {
		return ErrorGrammar
	} else if p.tt == AtKeywordToken {
		return p.parseAtRule()
	} else if p.tt == IdentToken || p.tt == DelimToken {
		return p.parseDeclaration()
	} else if p.tt == CustomPropertyNameToken {
		return p.parseCustomProperty()
	}

	// parse error
	p.initBuf()
	p.l.r.Move(-len(p.data))
	p.err = parse.NewErrorLexer(p.l.r, "CSS parse error: unexpected token '%s' in declaration", string(p.data))
	p.l.r.Move(len(p.data))

	if p.tt == RightBraceToken {
		// right brace token will occur when we've had a decl error that ended in a right brace token
		// as these are not handled by decl error, we handle it here explictly. Normally its used to end eg. the qual rule.
		p.pushBuf(p.tt, p.data)
		return ErrorGrammar
	}
	return p.parseDeclarationError(p.tt, p.data)
}

////////////////////////////////////////////////////////////////

func (p *Parser) parseAtRule() GrammarType {
	p.initBuf()
	parse.ToLower(p.data)
	atRuleName := p.data
	if len(atRuleName) > 0 && atRuleName[1] == '-' {
		if i := bytes.IndexByte(atRuleName[2:], '-'); i != -1 {
			atRuleName = atRuleName[i+2:] // skip vendor specific prefix
		}
	}
	atRule := ToHash(atRuleName[1:])

	first := true
	skipWS := false
	for {
		tt, data := p.popToken(false)
		if tt == LeftBraceToken && p.level == 0 {
			if atRule == Font_Face || atRule == Page {
				p.state = append(p.state, (*Parser).parseAtRuleDeclarationList)
			} else if atRule == Document || atRule == Keyframes || atRule == Media || atRule == Supports {
				p.state = append(p.state, (*Parser).parseAtRuleRuleList)
			} else {
				p.state = append(p.state, (*Parser).parseAtRuleUnknown)
			}
			return BeginAtRuleGrammar
		} else if (tt == SemicolonToken || tt == RightBraceToken) && p.level == 0 || tt == ErrorToken {
			p.prevEnd = (tt == RightBraceToken)
			return AtRuleGrammar
		} else if tt == LeftParenthesisToken || tt == LeftBraceToken || tt == LeftBracketToken || tt == FunctionToken {
			p.level++
		} else if tt == RightParenthesisToken || tt == RightBraceToken || tt == RightBracketToken {
			p.level--
		}
		if first {
			if tt == LeftParenthesisToken || tt == LeftBracketToken {
				p.prevWS = false
			}
			first = false
		}
		if len(data) == 1 && (data[0] == ',' || data[0] == ':') {
			skipWS = true
		} else if p.prevWS && !skipWS && tt != RightParenthesisToken {
			p.pushBuf(WhitespaceToken, wsBytes)
		} else {
			skipWS = false
		}
		if tt == LeftParenthesisToken {
			skipWS = true
		}
		p.pushBuf(tt, data)
	}
}

func (p *Parser) parseAtRuleRuleList() GrammarType {
	if p.tt == RightBraceToken || p.tt == ErrorToken {
		p.state = p.state[:len(p.state)-1]
		return EndAtRuleGrammar
	} else if p.tt == AtKeywordToken {
		return p.parseAtRule()
	} else {
		return p.parseQualifiedRule()
	}
}

func (p *Parser) parseAtRuleDeclarationList() GrammarType {
	for p.tt == SemicolonToken {
		p.tt, p.data = p.popToken(false)
	}
	if p.tt == RightBraceToken || p.tt == ErrorToken {
		p.state = p.state[:len(p.state)-1]
		return EndAtRuleGrammar
	}
	return p.parseDeclarationList()
}

func (p *Parser) parseAtRuleUnknown() GrammarType {
	p.keepWS = true
	if p.tt == RightBraceToken && p.level == 0 || p.tt == ErrorToken {
		p.state = p.state[:len(p.state)-1]
		p.keepWS = false
		return EndAtRuleGrammar
	}
	if p.tt == LeftParenthesisToken || p.tt == LeftBraceToken || p.tt == LeftBracketToken || p.tt == FunctionToken {
		p.level++
	} else if p.tt == RightParenthesisToken || p.tt == RightBraceToken || p.tt == RightBracketToken {
		p.level--
	}
	return TokenGrammar
}

func (p *Parser) parseQualifiedRule() GrammarType {
	p.initBuf()
	first := true
	inAttrSel := false
	skipWS := true
	var tt TokenType
	var data []byte
	for {
		if first {
			tt, data = p.tt, p.data
			p.tt = WhitespaceToken
			p.data = emptyBytes
			first = false
		} else {
			tt, data = p.popToken(false)
		}
		if tt == LeftBraceToken && p.level == 0 {
			p.state = append(p.state, (*Parser).parseQualifiedRuleDeclarationList)
			return BeginRulesetGrammar
		} else if tt == ErrorToken {
			p.err = parse.NewErrorLexer(p.l.r, "CSS parse error: unexpected ending in qualified rule")
			return ErrorGrammar
		} else if tt == LeftParenthesisToken || tt == LeftBraceToken || tt == LeftBracketToken || tt == FunctionToken {
			p.level++
		} else if tt == RightParenthesisToken || tt == RightBraceToken || tt == RightBracketToken {
			p.level--
		}
		if len(data) == 1 && (data[0] == ',' || data[0] == '>' || data[0] == '+' || data[0] == '~') {
			if data[0] == ',' {
				return QualifiedRuleGrammar
			}
			skipWS = true
		} else if p.prevWS && !skipWS && !inAttrSel {
			p.pushBuf(WhitespaceToken, wsBytes)
		} else {
			skipWS = false
		}
		if tt == LeftBracketToken {
			inAttrSel = true
		} else if tt == RightBracketToken {
			inAttrSel = false
		}
		p.pushBuf(tt, data)
	}
}

func (p *Parser) parseQualifiedRuleDeclarationList() GrammarType {
	for p.tt == SemicolonToken {
		p.tt, p.data = p.popToken(false)
	}
	if p.tt == RightBraceToken || p.tt == ErrorToken {
		p.state = p.state[:len(p.state)-1]
		return EndRulesetGrammar
	}
	return p.parseDeclarationList()
}

func (p *Parser) parseDeclaration() GrammarType {
	p.initBuf()
	parse.ToLower(p.data)

	ttName, dataName := p.tt, p.data
	tt, data := p.popToken(false)
	if tt != ColonToken {
		p.l.r.Move(-len(data))
		p.err = parse.NewErrorLexer(p.l.r, "CSS parse error: expected colon in declaration")
		p.l.r.Move(len(data))
		p.pushBuf(ttName, dataName)
		return p.parseDeclarationError(tt, data)
	}

	skipWS := true
	for {
		tt, data := p.popToken(false)
		if (tt == SemicolonToken || tt == RightBraceToken) && p.level == 0 || tt == ErrorToken {
			p.prevEnd = (tt == RightBraceToken)
			return DeclarationGrammar
		} else if tt == LeftParenthesisToken || tt == LeftBraceToken || tt == LeftBracketToken || tt == FunctionToken {
			p.level++
		} else if tt == RightParenthesisToken || tt == RightBraceToken || tt == RightBracketToken {
			p.level--
		}
		if len(data) == 1 && (data[0] == ',' || data[0] == '/' || data[0] == ':' || data[0] == '!' || data[0] == '=') {
			skipWS = true
		} else if (p.prevWS || p.prevComment) && !skipWS {
			p.pushBuf(WhitespaceToken, wsBytes)
		} else {
			skipWS = false
		}
		p.pushBuf(tt, data)
	}
}

func (p *Parser) parseDeclarationError(tt TokenType, data []byte) GrammarType {
	// we're on the offending (tt,data), keep popping tokens till we reach ;, }, or EOF
	p.tt, p.data = tt, data
	for {
		if (tt == SemicolonToken || tt == RightBraceToken) && p.level == 0 || tt == ErrorToken {
			p.prevEnd = (tt == RightBraceToken)
			if tt == SemicolonToken {
				p.pushBuf(tt, data)
			}
			return ErrorGrammar
		} else if tt == LeftParenthesisToken || tt == LeftBraceToken || tt == LeftBracketToken || tt == FunctionToken {
			p.level++
		} else if tt == RightParenthesisToken || tt == RightBraceToken || tt == RightBracketToken {
			p.level--
		}

		if p.prevWS {
			p.pushBuf(WhitespaceToken, wsBytes)
		}
		p.pushBuf(tt, data)

		tt, data = p.popToken(false)
	}
}

func (p *Parser) parseCustomProperty() GrammarType {
	p.initBuf()
	if tt, data := p.popToken(false); tt != ColonToken {
		p.l.r.Move(-len(data))
		p.err = parse.NewErrorLexer(p.l.r, "CSS parse error: expected colon in custom property")
		p.l.r.Move(len(data))
		return ErrorGrammar
	}
	val := []byte{}
	for {
		tt, data := p.l.Next()
		if (tt == SemicolonToken || tt == RightBraceToken) && p.level == 0 || tt == ErrorToken {
			p.prevEnd = (tt == RightBraceToken)
			p.pushBuf(CustomPropertyValueToken, val)
			return CustomPropertyGrammar
		} else if tt == LeftParenthesisToken || tt == LeftBraceToken || tt == LeftBracketToken || tt == FunctionToken {
			p.level++
		} else if tt == RightParenthesisToken || tt == RightBraceToken || tt == RightBracketToken {
			p.level--
		}
		val = append(val, data...)
	}
}
