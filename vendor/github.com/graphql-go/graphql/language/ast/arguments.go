package ast

import (
	"github.com/graphql-go/graphql/language/kinds"
)

// Argument implements Node
type Argument struct {
	Kind  string
	Loc   *Location
	Name  *Name
	Value Value
}

func NewArgument(arg *Argument) *Argument {
	if arg == nil {
		arg = &Argument{}
	}
	arg.Kind = kinds.Argument
	return arg
}

func (arg *Argument) GetKind() string {
	return arg.Kind
}

func (arg *Argument) GetLoc() *Location {
	return arg.Loc
}
