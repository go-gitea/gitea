package ast

import (
	"github.com/graphql-go/graphql/language/kinds"
)

type Definition interface {
	GetOperation() string
	GetVariableDefinitions() []*VariableDefinition
	GetSelectionSet() *SelectionSet
	GetKind() string
	GetLoc() *Location
}

// Ensure that all definition types implements Definition interface
var _ Definition = (*OperationDefinition)(nil)
var _ Definition = (*FragmentDefinition)(nil)
var _ Definition = (TypeSystemDefinition)(nil) // experimental non-spec addition.

// Note: subscription is an experimental non-spec addition.
const (
	OperationTypeQuery        = "query"
	OperationTypeMutation     = "mutation"
	OperationTypeSubscription = "subscription"
)

// OperationDefinition implements Node, Definition
type OperationDefinition struct {
	Kind                string
	Loc                 *Location
	Operation           string
	Name                *Name
	VariableDefinitions []*VariableDefinition
	Directives          []*Directive
	SelectionSet        *SelectionSet
}

func NewOperationDefinition(op *OperationDefinition) *OperationDefinition {
	if op == nil {
		op = &OperationDefinition{}
	}
	op.Kind = kinds.OperationDefinition
	return op
}

func (op *OperationDefinition) GetKind() string {
	return op.Kind
}

func (op *OperationDefinition) GetLoc() *Location {
	return op.Loc
}

func (op *OperationDefinition) GetOperation() string {
	return op.Operation
}

func (op *OperationDefinition) GetName() *Name {
	return op.Name
}

func (op *OperationDefinition) GetVariableDefinitions() []*VariableDefinition {
	return op.VariableDefinitions
}

func (op *OperationDefinition) GetDirectives() []*Directive {
	return op.Directives
}

func (op *OperationDefinition) GetSelectionSet() *SelectionSet {
	return op.SelectionSet
}

// FragmentDefinition implements Node, Definition
type FragmentDefinition struct {
	Kind                string
	Loc                 *Location
	Operation           string
	Name                *Name
	VariableDefinitions []*VariableDefinition
	TypeCondition       *Named
	Directives          []*Directive
	SelectionSet        *SelectionSet
}

func NewFragmentDefinition(fd *FragmentDefinition) *FragmentDefinition {
	if fd == nil {
		fd = &FragmentDefinition{}
	}
	return &FragmentDefinition{
		Kind:                kinds.FragmentDefinition,
		Loc:                 fd.Loc,
		Operation:           fd.Operation,
		Name:                fd.Name,
		VariableDefinitions: fd.VariableDefinitions,
		TypeCondition:       fd.TypeCondition,
		Directives:          fd.Directives,
		SelectionSet:        fd.SelectionSet,
	}
}

func (fd *FragmentDefinition) GetKind() string {
	return fd.Kind
}

func (fd *FragmentDefinition) GetLoc() *Location {
	return fd.Loc
}

func (fd *FragmentDefinition) GetOperation() string {
	return fd.Operation
}

func (fd *FragmentDefinition) GetName() *Name {
	return fd.Name
}

func (fd *FragmentDefinition) GetVariableDefinitions() []*VariableDefinition {
	return fd.VariableDefinitions
}

func (fd *FragmentDefinition) GetSelectionSet() *SelectionSet {
	return fd.SelectionSet
}

// VariableDefinition implements Node
type VariableDefinition struct {
	Kind         string
	Loc          *Location
	Variable     *Variable
	Type         Type
	DefaultValue Value
}

func NewVariableDefinition(vd *VariableDefinition) *VariableDefinition {
	if vd == nil {
		vd = &VariableDefinition{}
	}
	vd.Kind = kinds.VariableDefinition
	return vd
}

func (vd *VariableDefinition) GetKind() string {
	return vd.Kind
}

func (vd *VariableDefinition) GetLoc() *Location {
	return vd.Loc
}

// TypeExtensionDefinition implements Node, Definition
type TypeExtensionDefinition struct {
	Kind       string
	Loc        *Location
	Definition *ObjectDefinition
}

func NewTypeExtensionDefinition(def *TypeExtensionDefinition) *TypeExtensionDefinition {
	if def == nil {
		def = &TypeExtensionDefinition{}
	}
	return &TypeExtensionDefinition{
		Kind:       kinds.TypeExtensionDefinition,
		Loc:        def.Loc,
		Definition: def.Definition,
	}
}

func (def *TypeExtensionDefinition) GetKind() string {
	return def.Kind
}

func (def *TypeExtensionDefinition) GetLoc() *Location {
	return def.Loc
}

func (def *TypeExtensionDefinition) GetVariableDefinitions() []*VariableDefinition {
	return []*VariableDefinition{}
}

func (def *TypeExtensionDefinition) GetSelectionSet() *SelectionSet {
	return &SelectionSet{}
}

func (def *TypeExtensionDefinition) GetOperation() string {
	return ""
}

// DirectiveDefinition implements Node, Definition
type DirectiveDefinition struct {
	Kind        string
	Loc         *Location
	Name        *Name
	Description *StringValue
	Arguments   []*InputValueDefinition
	Locations   []*Name
}

func NewDirectiveDefinition(def *DirectiveDefinition) *DirectiveDefinition {
	if def == nil {
		def = &DirectiveDefinition{}
	}
	return &DirectiveDefinition{
		Kind:        kinds.DirectiveDefinition,
		Loc:         def.Loc,
		Name:        def.Name,
		Description: def.Description,
		Arguments:   def.Arguments,
		Locations:   def.Locations,
	}
}

func (def *DirectiveDefinition) GetKind() string {
	return def.Kind
}

func (def *DirectiveDefinition) GetLoc() *Location {
	return def.Loc
}

func (def *DirectiveDefinition) GetVariableDefinitions() []*VariableDefinition {
	return []*VariableDefinition{}
}

func (def *DirectiveDefinition) GetSelectionSet() *SelectionSet {
	return &SelectionSet{}
}

func (def *DirectiveDefinition) GetOperation() string {
	return ""
}

func (def *DirectiveDefinition) GetDescription() *StringValue {
	return def.Description
}
