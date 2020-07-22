package ast

import (
	"github.com/graphql-go/graphql/language/kinds"
)

// Document implements Node
type Document struct {
	Kind        string
	Loc         *Location
	Definitions []Node
}

func NewDocument(d *Document) *Document {
	if d == nil {
		d = &Document{}
	}
	return &Document{
		Kind:        kinds.Document,
		Loc:         d.Loc,
		Definitions: d.Definitions,
	}
}

func (node *Document) GetKind() string {
	return node.Kind
}

func (node *Document) GetLoc() *Location {
	return node.Loc
}
