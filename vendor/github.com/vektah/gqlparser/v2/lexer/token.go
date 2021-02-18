package lexer

import (
	"strconv"

	"github.com/vektah/gqlparser/v2/ast"
)

const (
	Invalid Type = iota
	EOF
	Bang
	Dollar
	Amp
	ParenL
	ParenR
	Spread
	Colon
	Equals
	At
	BracketL
	BracketR
	BraceL
	BraceR
	Pipe
	Name
	Int
	Float
	String
	BlockString
	Comment
)

func (t Type) Name() string {
	switch t {
	case Invalid:
		return "Invalid"
	case EOF:
		return "EOF"
	case Bang:
		return "Bang"
	case Dollar:
		return "Dollar"
	case Amp:
		return "Amp"
	case ParenL:
		return "ParenL"
	case ParenR:
		return "ParenR"
	case Spread:
		return "Spread"
	case Colon:
		return "Colon"
	case Equals:
		return "Equals"
	case At:
		return "At"
	case BracketL:
		return "BracketL"
	case BracketR:
		return "BracketR"
	case BraceL:
		return "BraceL"
	case BraceR:
		return "BraceR"
	case Pipe:
		return "Pipe"
	case Name:
		return "Name"
	case Int:
		return "Int"
	case Float:
		return "Float"
	case String:
		return "String"
	case BlockString:
		return "BlockString"
	case Comment:
		return "Comment"
	}
	return "Unknown " + strconv.Itoa(int(t))
}

func (t Type) String() string {
	switch t {
	case Invalid:
		return "<Invalid>"
	case EOF:
		return "<EOF>"
	case Bang:
		return "!"
	case Dollar:
		return "$"
	case Amp:
		return "&"
	case ParenL:
		return "("
	case ParenR:
		return ")"
	case Spread:
		return "..."
	case Colon:
		return ":"
	case Equals:
		return "="
	case At:
		return "@"
	case BracketL:
		return "["
	case BracketR:
		return "]"
	case BraceL:
		return "{"
	case BraceR:
		return "}"
	case Pipe:
		return "|"
	case Name:
		return "Name"
	case Int:
		return "Int"
	case Float:
		return "Float"
	case String:
		return "String"
	case BlockString:
		return "BlockString"
	case Comment:
		return "Comment"
	}
	return "Unknown " + strconv.Itoa(int(t))
}

// Kind represents a type of token. The types are predefined as constants.
type Type int

type Token struct {
	Kind  Type         // The token type.
	Value string       // The literal value consumed.
	Pos   ast.Position // The file and line this token was read from
}

func (t Token) String() string {
	if t.Value != "" {
		return t.Kind.String() + " " + strconv.Quote(t.Value)
	}
	return t.Kind.String()
}
