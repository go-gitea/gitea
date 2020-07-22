package ast

import (
	"github.com/graphql-go/graphql/language/kinds"
)

type Type interface {
	GetKind() string
	GetLoc() *Location
	String() string
}

// Ensure that all value types implements Value interface
var _ Type = (*Named)(nil)
var _ Type = (*List)(nil)
var _ Type = (*NonNull)(nil)

// Named implements Node, Type
type Named struct {
	Kind string
	Loc  *Location
	Name *Name
}

func NewNamed(t *Named) *Named {
	if t == nil {
		t = &Named{}
	}
	t.Kind = kinds.Named
	return t
}

func (t *Named) GetKind() string {
	return t.Kind
}

func (t *Named) GetLoc() *Location {
	return t.Loc
}

func (t *Named) String() string {
	return t.GetKind()
}

// List implements Node, Type
type List struct {
	Kind string
	Loc  *Location
	Type Type
}

func NewList(t *List) *List {
	if t == nil {
		t = &List{}
	}
	return &List{
		Kind: kinds.List,
		Loc:  t.Loc,
		Type: t.Type,
	}
}

func (t *List) GetKind() string {
	return t.Kind
}

func (t *List) GetLoc() *Location {
	return t.Loc
}

func (t *List) String() string {
	return t.GetKind()
}

// NonNull implements Node, Type
type NonNull struct {
	Kind string
	Loc  *Location
	Type Type
}

func NewNonNull(t *NonNull) *NonNull {
	if t == nil {
		t = &NonNull{}
	}
	return &NonNull{
		Kind: kinds.NonNull,
		Loc:  t.Loc,
		Type: t.Type,
	}
}

func (t *NonNull) GetKind() string {
	return t.Kind
}

func (t *NonNull) GetLoc() *Location {
	return t.Loc
}

func (t *NonNull) String() string {
	return t.GetKind()
}
