package ast

import (
	"github.com/graphql-go/graphql/language/kinds"
)

// Directive implements Node
type Directive struct {
	Kind      string
	Loc       *Location
	Name      *Name
	Arguments []*Argument
}

func NewDirective(dir *Directive) *Directive {
	if dir == nil {
		dir = &Directive{}
	}
	return &Directive{
		Kind:      kinds.Directive,
		Loc:       dir.Loc,
		Name:      dir.Name,
		Arguments: dir.Arguments,
	}
}

func (dir *Directive) GetKind() string {
	return dir.Kind
}

func (dir *Directive) GetLoc() *Location {
	return dir.Loc
}
