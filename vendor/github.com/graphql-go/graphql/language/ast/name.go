package ast

import (
	"github.com/graphql-go/graphql/language/kinds"
)

// Name implements Node
type Name struct {
	Kind  string
	Loc   *Location
	Value string
}

func NewName(node *Name) *Name {
	if node == nil {
		node = &Name{}
	}
	node.Kind = kinds.Name
	return node
}

func (node *Name) GetKind() string {
	return node.Kind
}

func (node *Name) GetLoc() *Location {
	return node.Loc
}
