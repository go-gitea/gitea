package ast

import (
	"github.com/graphql-go/graphql/language/kinds"
)

type Selection interface {
	GetSelectionSet() *SelectionSet
}

// Ensure that all definition types implements Selection interface
var _ Selection = (*Field)(nil)
var _ Selection = (*FragmentSpread)(nil)
var _ Selection = (*InlineFragment)(nil)

// Field implements Node, Selection
type Field struct {
	Kind         string
	Loc          *Location
	Alias        *Name
	Name         *Name
	Arguments    []*Argument
	Directives   []*Directive
	SelectionSet *SelectionSet
}

func NewField(f *Field) *Field {
	if f == nil {
		f = &Field{}
	}
	f.Kind = kinds.Field
	return f
}

func (f *Field) GetKind() string {
	return f.Kind
}

func (f *Field) GetLoc() *Location {
	return f.Loc
}

func (f *Field) GetSelectionSet() *SelectionSet {
	return f.SelectionSet
}

// FragmentSpread implements Node, Selection
type FragmentSpread struct {
	Kind       string
	Loc        *Location
	Name       *Name
	Directives []*Directive
}

func NewFragmentSpread(fs *FragmentSpread) *FragmentSpread {
	if fs == nil {
		fs = &FragmentSpread{}
	}
	return &FragmentSpread{
		Kind:       kinds.FragmentSpread,
		Loc:        fs.Loc,
		Name:       fs.Name,
		Directives: fs.Directives,
	}
}

func (fs *FragmentSpread) GetKind() string {
	return fs.Kind
}

func (fs *FragmentSpread) GetLoc() *Location {
	return fs.Loc
}

func (fs *FragmentSpread) GetSelectionSet() *SelectionSet {
	return nil
}

// InlineFragment implements Node, Selection
type InlineFragment struct {
	Kind          string
	Loc           *Location
	TypeCondition *Named
	Directives    []*Directive
	SelectionSet  *SelectionSet
}

func NewInlineFragment(f *InlineFragment) *InlineFragment {
	if f == nil {
		f = &InlineFragment{}
	}
	return &InlineFragment{
		Kind:          kinds.InlineFragment,
		Loc:           f.Loc,
		TypeCondition: f.TypeCondition,
		Directives:    f.Directives,
		SelectionSet:  f.SelectionSet,
	}
}

func (f *InlineFragment) GetKind() string {
	return f.Kind
}

func (f *InlineFragment) GetLoc() *Location {
	return f.Loc
}

func (f *InlineFragment) GetSelectionSet() *SelectionSet {
	return f.SelectionSet
}

// SelectionSet implements Node
type SelectionSet struct {
	Kind       string
	Loc        *Location
	Selections []Selection
}

func NewSelectionSet(ss *SelectionSet) *SelectionSet {
	if ss == nil {
		ss = &SelectionSet{}
	}
	ss.Kind = kinds.SelectionSet
	return ss
}

func (ss *SelectionSet) GetKind() string {
	return ss.Kind
}

func (ss *SelectionSet) GetLoc() *Location {
	return ss.Loc
}
