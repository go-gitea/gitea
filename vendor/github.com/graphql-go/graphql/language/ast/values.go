package ast

import (
	"github.com/graphql-go/graphql/language/kinds"
)

type Value interface {
	GetValue() interface{}
	GetKind() string
	GetLoc() *Location
}

// Ensure that all value types implements Value interface
var _ Value = (*Variable)(nil)
var _ Value = (*IntValue)(nil)
var _ Value = (*FloatValue)(nil)
var _ Value = (*StringValue)(nil)
var _ Value = (*BooleanValue)(nil)
var _ Value = (*EnumValue)(nil)
var _ Value = (*ListValue)(nil)
var _ Value = (*ObjectValue)(nil)

// Variable implements Node, Value
type Variable struct {
	Kind string
	Loc  *Location
	Name *Name
}

func NewVariable(v *Variable) *Variable {
	if v == nil {
		v = &Variable{}
	}
	v.Kind = kinds.Variable
	return v
}

func (v *Variable) GetKind() string {
	return v.Kind
}

func (v *Variable) GetLoc() *Location {
	return v.Loc
}

// GetValue alias to Variable.GetName()
func (v *Variable) GetValue() interface{} {
	return v.GetName()
}

func (v *Variable) GetName() interface{} {
	return v.Name
}

// IntValue implements Node, Value
type IntValue struct {
	Kind  string
	Loc   *Location
	Value string
}

func NewIntValue(v *IntValue) *IntValue {
	if v == nil {
		v = &IntValue{}
	}
	return &IntValue{
		Kind:  kinds.IntValue,
		Loc:   v.Loc,
		Value: v.Value,
	}
}

func (v *IntValue) GetKind() string {
	return v.Kind
}

func (v *IntValue) GetLoc() *Location {
	return v.Loc
}

func (v *IntValue) GetValue() interface{} {
	return v.Value
}

// FloatValue implements Node, Value
type FloatValue struct {
	Kind  string
	Loc   *Location
	Value string
}

func NewFloatValue(v *FloatValue) *FloatValue {
	if v == nil {
		v = &FloatValue{}
	}
	return &FloatValue{
		Kind:  kinds.FloatValue,
		Loc:   v.Loc,
		Value: v.Value,
	}
}

func (v *FloatValue) GetKind() string {
	return v.Kind
}

func (v *FloatValue) GetLoc() *Location {
	return v.Loc
}

func (v *FloatValue) GetValue() interface{} {
	return v.Value
}

// StringValue implements Node, Value
type StringValue struct {
	Kind  string
	Loc   *Location
	Value string
}

func NewStringValue(v *StringValue) *StringValue {
	if v == nil {
		v = &StringValue{}
	}
	return &StringValue{
		Kind:  kinds.StringValue,
		Loc:   v.Loc,
		Value: v.Value,
	}
}

func (v *StringValue) GetKind() string {
	return v.Kind
}

func (v *StringValue) GetLoc() *Location {
	return v.Loc
}

func (v *StringValue) GetValue() interface{} {
	return v.Value
}

// BooleanValue implements Node, Value
type BooleanValue struct {
	Kind  string
	Loc   *Location
	Value bool
}

func NewBooleanValue(v *BooleanValue) *BooleanValue {
	if v == nil {
		v = &BooleanValue{}
	}
	return &BooleanValue{
		Kind:  kinds.BooleanValue,
		Loc:   v.Loc,
		Value: v.Value,
	}
}

func (v *BooleanValue) GetKind() string {
	return v.Kind
}

func (v *BooleanValue) GetLoc() *Location {
	return v.Loc
}

func (v *BooleanValue) GetValue() interface{} {
	return v.Value
}

// EnumValue implements Node, Value
type EnumValue struct {
	Kind  string
	Loc   *Location
	Value string
}

func NewEnumValue(v *EnumValue) *EnumValue {
	if v == nil {
		v = &EnumValue{}
	}
	return &EnumValue{
		Kind:  kinds.EnumValue,
		Loc:   v.Loc,
		Value: v.Value,
	}
}

func (v *EnumValue) GetKind() string {
	return v.Kind
}

func (v *EnumValue) GetLoc() *Location {
	return v.Loc
}

func (v *EnumValue) GetValue() interface{} {
	return v.Value
}

// ListValue implements Node, Value
type ListValue struct {
	Kind   string
	Loc    *Location
	Values []Value
}

func NewListValue(v *ListValue) *ListValue {
	if v == nil {
		v = &ListValue{}
	}
	return &ListValue{
		Kind:   kinds.ListValue,
		Loc:    v.Loc,
		Values: v.Values,
	}
}

func (v *ListValue) GetKind() string {
	return v.Kind
}

func (v *ListValue) GetLoc() *Location {
	return v.Loc
}

// GetValue alias to ListValue.GetValues()
func (v *ListValue) GetValue() interface{} {
	return v.GetValues()
}

func (v *ListValue) GetValues() interface{} {
	// TODO: verify ObjectValue.GetValue()
	return v.Values
}

// ObjectValue implements Node, Value
type ObjectValue struct {
	Kind   string
	Loc    *Location
	Fields []*ObjectField
}

func NewObjectValue(v *ObjectValue) *ObjectValue {
	if v == nil {
		v = &ObjectValue{}
	}
	return &ObjectValue{
		Kind:   kinds.ObjectValue,
		Loc:    v.Loc,
		Fields: v.Fields,
	}
}

func (v *ObjectValue) GetKind() string {
	return v.Kind
}

func (v *ObjectValue) GetLoc() *Location {
	return v.Loc
}

func (v *ObjectValue) GetValue() interface{} {
	// TODO: verify ObjectValue.GetValue()
	return v.Fields
}

// ObjectField implements Node, Value
type ObjectField struct {
	Kind  string
	Name  *Name
	Loc   *Location
	Value Value
}

func NewObjectField(f *ObjectField) *ObjectField {
	if f == nil {
		f = &ObjectField{}
	}
	return &ObjectField{
		Kind:  kinds.ObjectField,
		Loc:   f.Loc,
		Name:  f.Name,
		Value: f.Value,
	}
}

func (f *ObjectField) GetKind() string {
	return f.Kind
}

func (f *ObjectField) GetLoc() *Location {
	return f.Loc
}

func (f *ObjectField) GetValue() interface{} {
	return f.Value
}
