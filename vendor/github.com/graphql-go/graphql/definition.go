package graphql

import (
	"context"
	"fmt"
	"reflect"
	"regexp"

	"github.com/graphql-go/graphql/language/ast"
)

// Type interface for all of the possible kinds of GraphQL types
type Type interface {
	Name() string
	Description() string
	String() string
	Error() error
}

var _ Type = (*Scalar)(nil)
var _ Type = (*Object)(nil)
var _ Type = (*Interface)(nil)
var _ Type = (*Union)(nil)
var _ Type = (*Enum)(nil)
var _ Type = (*InputObject)(nil)
var _ Type = (*List)(nil)
var _ Type = (*NonNull)(nil)
var _ Type = (*Argument)(nil)

// Input interface for types that may be used as input types for arguments and directives.
type Input interface {
	Name() string
	Description() string
	String() string
	Error() error
}

var _ Input = (*Scalar)(nil)
var _ Input = (*Enum)(nil)
var _ Input = (*InputObject)(nil)
var _ Input = (*List)(nil)
var _ Input = (*NonNull)(nil)

// IsInputType determines if given type is a GraphQLInputType
func IsInputType(ttype Type) bool {
	switch GetNamed(ttype).(type) {
	case *Scalar, *Enum, *InputObject:
		return true
	default:
		return false
	}
}

// IsOutputType determines if given type is a GraphQLOutputType
func IsOutputType(ttype Type) bool {
	switch GetNamed(ttype).(type) {
	case *Scalar, *Object, *Interface, *Union, *Enum:
		return true
	default:
		return false
	}
}

// Leaf interface for types that may be leaf values
type Leaf interface {
	Name() string
	Description() string
	String() string
	Error() error
	Serialize(value interface{}) interface{}
}

var _ Leaf = (*Scalar)(nil)
var _ Leaf = (*Enum)(nil)

// IsLeafType determines if given type is a leaf value
func IsLeafType(ttype Type) bool {
	switch GetNamed(ttype).(type) {
	case *Scalar, *Enum:
		return true
	default:
		return false
	}
}

// Output interface for types that may be used as output types as the result of fields.
type Output interface {
	Name() string
	Description() string
	String() string
	Error() error
}

var _ Output = (*Scalar)(nil)
var _ Output = (*Object)(nil)
var _ Output = (*Interface)(nil)
var _ Output = (*Union)(nil)
var _ Output = (*Enum)(nil)
var _ Output = (*List)(nil)
var _ Output = (*NonNull)(nil)

// Composite interface for types that may describe the parent context of a selection set.
type Composite interface {
	Name() string
	Description() string
	String() string
	Error() error
}

var _ Composite = (*Object)(nil)
var _ Composite = (*Interface)(nil)
var _ Composite = (*Union)(nil)

// IsCompositeType determines if given type is a GraphQLComposite type
func IsCompositeType(ttype interface{}) bool {
	switch ttype.(type) {
	case *Object, *Interface, *Union:
		return true
	default:
		return false
	}
}

// Abstract interface for types that may describe the parent context of a selection set.
type Abstract interface {
	Name() string
}

var _ Abstract = (*Interface)(nil)
var _ Abstract = (*Union)(nil)

func IsAbstractType(ttype interface{}) bool {
	switch ttype.(type) {
	case *Interface, *Union:
		return true
	default:
		return false
	}
}

// Nullable interface for types that can accept null as a value.
type Nullable interface {
}

var _ Nullable = (*Scalar)(nil)
var _ Nullable = (*Object)(nil)
var _ Nullable = (*Interface)(nil)
var _ Nullable = (*Union)(nil)
var _ Nullable = (*Enum)(nil)
var _ Nullable = (*InputObject)(nil)
var _ Nullable = (*List)(nil)

// GetNullable returns the Nullable type of the given GraphQL type
func GetNullable(ttype Type) Nullable {
	if ttype, ok := ttype.(*NonNull); ok {
		return ttype.OfType
	}
	return ttype
}

// Named interface for types that do not include modifiers like List or NonNull.
type Named interface {
	String() string
}

var _ Named = (*Scalar)(nil)
var _ Named = (*Object)(nil)
var _ Named = (*Interface)(nil)
var _ Named = (*Union)(nil)
var _ Named = (*Enum)(nil)
var _ Named = (*InputObject)(nil)

// GetNamed returns the Named type of the given GraphQL type
func GetNamed(ttype Type) Named {
	unmodifiedType := ttype
	for {
		switch typ := unmodifiedType.(type) {
		case *List:
			unmodifiedType = typ.OfType
		case *NonNull:
			unmodifiedType = typ.OfType
		default:
			return unmodifiedType
		}
	}
}

// Scalar Type Definition
//
// The leaf values of any request and input values to arguments are
// Scalars (or Enums) and are defined with a name and a series of functions
// used to parse input from ast or variables and to ensure validity.
//
// Example:
//
//    var OddType = new Scalar({
//      name: 'Odd',
//      serialize(value) {
//        return value % 2 === 1 ? value : null;
//      }
//    });
//
type Scalar struct {
	PrivateName        string `json:"name"`
	PrivateDescription string `json:"description"`

	scalarConfig ScalarConfig
	err          error
}

// SerializeFn is a function type for serializing a GraphQLScalar type value
type SerializeFn func(value interface{}) interface{}

// ParseValueFn is a function type for parsing the value of a GraphQLScalar type
type ParseValueFn func(value interface{}) interface{}

// ParseLiteralFn is a function type for parsing the literal value of a GraphQLScalar type
type ParseLiteralFn func(valueAST ast.Value) interface{}

// ScalarConfig options for creating a new GraphQLScalar
type ScalarConfig struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Serialize    SerializeFn
	ParseValue   ParseValueFn
	ParseLiteral ParseLiteralFn
}

// NewScalar creates a new GraphQLScalar
func NewScalar(config ScalarConfig) *Scalar {
	st := &Scalar{}
	err := invariant(config.Name != "", "Type must be named.")
	if err != nil {
		st.err = err
		return st
	}

	err = assertValidName(config.Name)
	if err != nil {
		st.err = err
		return st
	}

	st.PrivateName = config.Name
	st.PrivateDescription = config.Description

	err = invariantf(
		config.Serialize != nil,
		`%v must provide "serialize" function. If this custom Scalar is `+
			`also used as an input type, ensure "parseValue" and "parseLiteral" `+
			`functions are also provided.`, st,
	)
	if err != nil {
		st.err = err
		return st
	}
	if config.ParseValue != nil || config.ParseLiteral != nil {
		err = invariantf(
			config.ParseValue != nil && config.ParseLiteral != nil,
			`%v must provide both "parseValue" and "parseLiteral" functions.`, st,
		)
		if err != nil {
			st.err = err
			return st
		}
	}

	st.scalarConfig = config
	return st
}
func (st *Scalar) Serialize(value interface{}) interface{} {
	if st.scalarConfig.Serialize == nil {
		return value
	}
	return st.scalarConfig.Serialize(value)
}
func (st *Scalar) ParseValue(value interface{}) interface{} {
	if st.scalarConfig.ParseValue == nil {
		return value
	}
	return st.scalarConfig.ParseValue(value)
}
func (st *Scalar) ParseLiteral(valueAST ast.Value) interface{} {
	if st.scalarConfig.ParseLiteral == nil {
		return nil
	}
	return st.scalarConfig.ParseLiteral(valueAST)
}
func (st *Scalar) Name() string {
	return st.PrivateName
}
func (st *Scalar) Description() string {
	return st.PrivateDescription

}
func (st *Scalar) String() string {
	return st.PrivateName
}
func (st *Scalar) Error() error {
	return st.err
}

// Object Type Definition
//
// Almost all of the GraphQL types you define will be object  Object types
// have a name, but most importantly describe their fields.
// Example:
//
//    var AddressType = new Object({
//      name: 'Address',
//      fields: {
//        street: { type: String },
//        number: { type: Int },
//        formatted: {
//          type: String,
//          resolve(obj) {
//            return obj.number + ' ' + obj.street
//          }
//        }
//      }
//    });
//
// When two types need to refer to each other, or a type needs to refer to
// itself in a field, you can use a function expression (aka a closure or a
// thunk) to supply the fields lazily.
//
// Example:
//
//    var PersonType = new Object({
//      name: 'Person',
//      fields: () => ({
//        name: { type: String },
//        bestFriend: { type: PersonType },
//      })
//    });
//
// /
type Object struct {
	PrivateName        string `json:"name"`
	PrivateDescription string `json:"description"`
	IsTypeOf           IsTypeOfFn

	typeConfig            ObjectConfig
	initialisedFields     bool
	fields                FieldDefinitionMap
	initialisedInterfaces bool
	interfaces            []*Interface
	// Interim alternative to throwing an error during schema definition at run-time
	err error
}

// IsTypeOfParams Params for IsTypeOfFn()
type IsTypeOfParams struct {
	// Value that needs to be resolve.
	// Use this to decide which GraphQLObject this value maps to.
	Value interface{}

	// Info is a collection of information about the current execution state.
	Info ResolveInfo

	// Context argument is a context value that is provided to every resolve function within an execution.
	// It is commonly
	// used to represent an authenticated user, or request-specific caches.
	Context context.Context
}

type IsTypeOfFn func(p IsTypeOfParams) bool

type InterfacesThunk func() []*Interface

type ObjectConfig struct {
	Name        string      `json:"name"`
	Interfaces  interface{} `json:"interfaces"`
	Fields      interface{} `json:"fields"`
	IsTypeOf    IsTypeOfFn  `json:"isTypeOf"`
	Description string      `json:"description"`
}

type FieldsThunk func() Fields

func NewObject(config ObjectConfig) *Object {
	objectType := &Object{}

	err := invariant(config.Name != "", "Type must be named.")
	if err != nil {
		objectType.err = err
		return objectType
	}
	err = assertValidName(config.Name)
	if err != nil {
		objectType.err = err
		return objectType
	}

	objectType.PrivateName = config.Name
	objectType.PrivateDescription = config.Description
	objectType.IsTypeOf = config.IsTypeOf
	objectType.typeConfig = config

	return objectType
}

// ensureCache ensures that both fields and interfaces have been initialized properly,
// to prevent races.
func (gt *Object) ensureCache() {
	gt.Fields()
	gt.Interfaces()
}
func (gt *Object) AddFieldConfig(fieldName string, fieldConfig *Field) {
	if fieldName == "" || fieldConfig == nil {
		return
	}
	if fields, ok := gt.typeConfig.Fields.(Fields); ok {
		fields[fieldName] = fieldConfig
		gt.initialisedFields = false
	}
}
func (gt *Object) Name() string {
	return gt.PrivateName
}
func (gt *Object) Description() string {
	return ""
}
func (gt *Object) String() string {
	return gt.PrivateName
}
func (gt *Object) Fields() FieldDefinitionMap {
	if gt.initialisedFields {
		return gt.fields
	}

	var configureFields Fields
	switch fields := gt.typeConfig.Fields.(type) {
	case Fields:
		configureFields = fields
	case FieldsThunk:
		configureFields = fields()
	}

	gt.fields, gt.err = defineFieldMap(gt, configureFields)
	gt.initialisedFields = true
	return gt.fields
}

func (gt *Object) Interfaces() []*Interface {
	if gt.initialisedInterfaces {
		return gt.interfaces
	}

	var configInterfaces []*Interface
	switch iface := gt.typeConfig.Interfaces.(type) {
	case InterfacesThunk:
		configInterfaces = iface()
	case []*Interface:
		configInterfaces = iface
	case nil:
	default:
		gt.err = fmt.Errorf("Unknown Object.Interfaces type: %T", gt.typeConfig.Interfaces)
		gt.initialisedInterfaces = true
		return nil
	}

	gt.interfaces, gt.err = defineInterfaces(gt, configInterfaces)
	gt.initialisedInterfaces = true
	return gt.interfaces
}

func (gt *Object) Error() error {
	return gt.err
}

func defineInterfaces(ttype *Object, interfaces []*Interface) ([]*Interface, error) {
	ifaces := []*Interface{}

	if len(interfaces) == 0 {
		return ifaces, nil
	}
	for _, iface := range interfaces {
		err := invariantf(
			iface != nil,
			`%v may only implement Interface types, it cannot implement: %v.`, ttype, iface,
		)
		if err != nil {
			return ifaces, err
		}
		if iface.ResolveType != nil {
			err = invariantf(
				iface.ResolveType != nil,
				`Interface Type %v does not provide a "resolveType" function `+
					`and implementing Type %v does not provide a "isTypeOf" `+
					`function. There is no way to resolve this implementing type `+
					`during execution.`, iface, ttype,
			)
			if err != nil {
				return ifaces, err
			}
		}
		ifaces = append(ifaces, iface)
	}

	return ifaces, nil
}

func defineFieldMap(ttype Named, fieldMap Fields) (FieldDefinitionMap, error) {
	resultFieldMap := FieldDefinitionMap{}

	err := invariantf(
		len(fieldMap) > 0,
		`%v fields must be an object with field names as keys or a function which return such an object.`, ttype,
	)
	if err != nil {
		return resultFieldMap, err
	}

	for fieldName, field := range fieldMap {
		if field == nil {
			continue
		}
		err = invariantf(
			field.Type != nil,
			`%v.%v field type must be Output Type but got: %v.`, ttype, fieldName, field.Type,
		)
		if err != nil {
			return resultFieldMap, err
		}
		if field.Type.Error() != nil {
			return resultFieldMap, field.Type.Error()
		}
		if err = assertValidName(fieldName); err != nil {
			return resultFieldMap, err
		}
		fieldDef := &FieldDefinition{
			Name:              fieldName,
			Description:       field.Description,
			Type:              field.Type,
			Resolve:           field.Resolve,
			DeprecationReason: field.DeprecationReason,
		}

		fieldDef.Args = []*Argument{}
		for argName, arg := range field.Args {
			if err = assertValidName(argName); err != nil {
				return resultFieldMap, err
			}
			if err = invariantf(
				arg != nil,
				`%v.%v args must be an object with argument names as keys.`, ttype, fieldName,
			); err != nil {
				return resultFieldMap, err
			}
			if err = invariantf(
				arg.Type != nil,
				`%v.%v(%v:) argument type must be Input Type but got: %v.`, ttype, fieldName, argName, arg.Type,
			); err != nil {
				return resultFieldMap, err
			}
			fieldArg := &Argument{
				PrivateName:        argName,
				PrivateDescription: arg.Description,
				Type:               arg.Type,
				DefaultValue:       arg.DefaultValue,
			}
			fieldDef.Args = append(fieldDef.Args, fieldArg)
		}
		resultFieldMap[fieldName] = fieldDef
	}
	return resultFieldMap, nil
}

// ResolveParams Params for FieldResolveFn()
type ResolveParams struct {
	// Source is the source value
	Source interface{}

	// Args is a map of arguments for current GraphQL request
	Args map[string]interface{}

	// Info is a collection of information about the current execution state.
	Info ResolveInfo

	// Context argument is a context value that is provided to every resolve function within an execution.
	// It is commonly
	// used to represent an authenticated user, or request-specific caches.
	Context context.Context
}

type FieldResolveFn func(p ResolveParams) (interface{}, error)

type ResolveInfo struct {
	FieldName      string
	FieldASTs      []*ast.Field
	Path           *ResponsePath
	ReturnType     Output
	ParentType     Composite
	Schema         Schema
	Fragments      map[string]ast.Definition
	RootValue      interface{}
	Operation      ast.Definition
	VariableValues map[string]interface{}
}

type Fields map[string]*Field

type Field struct {
	Name              string              `json:"name"` // used by graphlql-relay
	Type              Output              `json:"type"`
	Args              FieldConfigArgument `json:"args"`
	Resolve           FieldResolveFn      `json:"-"`
	DeprecationReason string              `json:"deprecationReason"`
	Description       string              `json:"description"`
}

type FieldConfigArgument map[string]*ArgumentConfig

type ArgumentConfig struct {
	Type         Input       `json:"type"`
	DefaultValue interface{} `json:"defaultValue"`
	Description  string      `json:"description"`
}

type FieldDefinitionMap map[string]*FieldDefinition
type FieldDefinition struct {
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	Type              Output         `json:"type"`
	Args              []*Argument    `json:"args"`
	Resolve           FieldResolveFn `json:"-"`
	DeprecationReason string         `json:"deprecationReason"`
}

type FieldArgument struct {
	Name         string      `json:"name"`
	Type         Type        `json:"type"`
	DefaultValue interface{} `json:"defaultValue"`
	Description  string      `json:"description"`
}

type Argument struct {
	PrivateName        string      `json:"name"`
	Type               Input       `json:"type"`
	DefaultValue       interface{} `json:"defaultValue"`
	PrivateDescription string      `json:"description"`
}

func (st *Argument) Name() string {
	return st.PrivateName
}
func (st *Argument) Description() string {
	return st.PrivateDescription

}
func (st *Argument) String() string {
	return st.PrivateName
}
func (st *Argument) Error() error {
	return nil
}

// Interface Type Definition
//
// When a field can return one of a heterogeneous set of types, a Interface type
// is used to describe what types are possible, what fields are in common across
// all types, as well as a function to determine which type is actually used
// when the field is resolved.
//
// Example:
//
//     var EntityType = new Interface({
//       name: 'Entity',
//       fields: {
//         name: { type: String }
//       }
//     });
//
//
type Interface struct {
	PrivateName        string `json:"name"`
	PrivateDescription string `json:"description"`
	ResolveType        ResolveTypeFn

	typeConfig        InterfaceConfig
	initialisedFields bool
	fields            FieldDefinitionMap
	err               error
}
type InterfaceConfig struct {
	Name        string      `json:"name"`
	Fields      interface{} `json:"fields"`
	ResolveType ResolveTypeFn
	Description string `json:"description"`
}

// ResolveTypeParams Params for ResolveTypeFn()
type ResolveTypeParams struct {
	// Value that needs to be resolve.
	// Use this to decide which GraphQLObject this value maps to.
	Value interface{}

	// Info is a collection of information about the current execution state.
	Info ResolveInfo

	// Context argument is a context value that is provided to every resolve function within an execution.
	// It is commonly
	// used to represent an authenticated user, or request-specific caches.
	Context context.Context
}

type ResolveTypeFn func(p ResolveTypeParams) *Object

func NewInterface(config InterfaceConfig) *Interface {
	it := &Interface{}

	if it.err = invariant(config.Name != "", "Type must be named."); it.err != nil {
		return it
	}
	if it.err = assertValidName(config.Name); it.err != nil {
		return it
	}
	it.PrivateName = config.Name
	it.PrivateDescription = config.Description
	it.ResolveType = config.ResolveType
	it.typeConfig = config

	return it
}

func (it *Interface) AddFieldConfig(fieldName string, fieldConfig *Field) {
	if fieldName == "" || fieldConfig == nil {
		return
	}
	if fields, ok := it.typeConfig.Fields.(Fields); ok {
		fields[fieldName] = fieldConfig
		it.initialisedFields = false
	}
}

func (it *Interface) Name() string {
	return it.PrivateName
}

func (it *Interface) Description() string {
	return it.PrivateDescription
}

func (it *Interface) Fields() (fields FieldDefinitionMap) {
	if it.initialisedFields {
		return it.fields
	}

	var configureFields Fields
	switch fields := it.typeConfig.Fields.(type) {
	case Fields:
		configureFields = fields
	case FieldsThunk:
		configureFields = fields()
	}

	it.fields, it.err = defineFieldMap(it, configureFields)
	it.initialisedFields = true
	return it.fields
}

func (it *Interface) String() string {
	return it.PrivateName
}

func (it *Interface) Error() error {
	return it.err
}

// Union Type Definition
//
// When a field can return one of a heterogeneous set of types, a Union type
// is used to describe what types are possible as well as providing a function
// to determine which type is actually used when the field is resolved.
//
// Example:
//
//     var PetType = new Union({
//       name: 'Pet',
//       types: [ DogType, CatType ],
//       resolveType(value) {
//         if (value instanceof Dog) {
//           return DogType;
//         }
//         if (value instanceof Cat) {
//           return CatType;
//         }
//       }
//     });
type Union struct {
	PrivateName        string `json:"name"`
	PrivateDescription string `json:"description"`
	ResolveType        ResolveTypeFn

	typeConfig    UnionConfig
	types         []*Object
	possibleTypes map[string]bool

	err error
}
type UnionConfig struct {
	Name        string    `json:"name"`
	Types       []*Object `json:"types"`
	ResolveType ResolveTypeFn
	Description string `json:"description"`
}

func NewUnion(config UnionConfig) *Union {
	objectType := &Union{}

	if objectType.err = invariant(config.Name != "", "Type must be named."); objectType.err != nil {
		return objectType
	}
	if objectType.err = assertValidName(config.Name); objectType.err != nil {
		return objectType
	}
	objectType.PrivateName = config.Name
	objectType.PrivateDescription = config.Description
	objectType.ResolveType = config.ResolveType

	if objectType.err = invariantf(
		len(config.Types) > 0,
		`Must provide Array of types for Union %v.`, config.Name,
	); objectType.err != nil {
		return objectType
	}
	for _, ttype := range config.Types {
		if objectType.err = invariantf(
			ttype != nil,
			`%v may only contain Object types, it cannot contain: %v.`, objectType, ttype,
		); objectType.err != nil {
			return objectType
		}
		if objectType.ResolveType == nil {
			if objectType.err = invariantf(
				ttype.IsTypeOf != nil,
				`Union Type %v does not provide a "resolveType" function `+
					`and possible Type %v does not provide a "isTypeOf" `+
					`function. There is no way to resolve this possible type `+
					`during execution.`, objectType, ttype,
			); objectType.err != nil {
				return objectType
			}
		}
	}
	objectType.types = config.Types
	objectType.typeConfig = config

	return objectType
}
func (ut *Union) Types() []*Object {
	return ut.types
}
func (ut *Union) String() string {
	return ut.PrivateName
}
func (ut *Union) Name() string {
	return ut.PrivateName
}
func (ut *Union) Description() string {
	return ut.PrivateDescription
}
func (ut *Union) Error() error {
	return ut.err
}

// Enum Type Definition
//
// Some leaf values of requests and input values are Enums. GraphQL serializes
// Enum values as strings, however internally Enums can be represented by any
// kind of type, often integers.
//
// Example:
//
//     var RGBType = new Enum({
//       name: 'RGB',
//       values: {
//         RED: { value: 0 },
//         GREEN: { value: 1 },
//         BLUE: { value: 2 }
//       }
//     });
//
// Note: If a value is not provided in a definition, the name of the enum value
// will be used as its internal value.

type Enum struct {
	PrivateName        string `json:"name"`
	PrivateDescription string `json:"description"`

	enumConfig   EnumConfig
	values       []*EnumValueDefinition
	valuesLookup map[interface{}]*EnumValueDefinition
	nameLookup   map[string]*EnumValueDefinition

	err error
}
type EnumValueConfigMap map[string]*EnumValueConfig
type EnumValueConfig struct {
	Value             interface{} `json:"value"`
	DeprecationReason string      `json:"deprecationReason"`
	Description       string      `json:"description"`
}
type EnumConfig struct {
	Name        string             `json:"name"`
	Values      EnumValueConfigMap `json:"values"`
	Description string             `json:"description"`
}
type EnumValueDefinition struct {
	Name              string      `json:"name"`
	Value             interface{} `json:"value"`
	DeprecationReason string      `json:"deprecationReason"`
	Description       string      `json:"description"`
}

func NewEnum(config EnumConfig) *Enum {
	gt := &Enum{}
	gt.enumConfig = config

	if gt.err = assertValidName(config.Name); gt.err != nil {
		return gt
	}

	gt.PrivateName = config.Name
	gt.PrivateDescription = config.Description
	if gt.values, gt.err = gt.defineEnumValues(config.Values); gt.err != nil {
		return gt
	}

	return gt
}
func (gt *Enum) defineEnumValues(valueMap EnumValueConfigMap) ([]*EnumValueDefinition, error) {
	var err error
	values := []*EnumValueDefinition{}

	if err = invariantf(
		len(valueMap) > 0,
		`%v values must be an object with value names as keys.`, gt,
	); err != nil {
		return values, err
	}

	for valueName, valueConfig := range valueMap {
		if err = invariantf(
			valueConfig != nil,
			`%v.%v must refer to an object with a "value" key `+
				`representing an internal value but got: %v.`, gt, valueName, valueConfig,
		); err != nil {
			return values, err
		}
		if err = assertValidName(valueName); err != nil {
			return values, err
		}
		value := &EnumValueDefinition{
			Name:              valueName,
			Value:             valueConfig.Value,
			DeprecationReason: valueConfig.DeprecationReason,
			Description:       valueConfig.Description,
		}
		if value.Value == nil {
			value.Value = valueName
		}
		values = append(values, value)
	}
	return values, nil
}
func (gt *Enum) Values() []*EnumValueDefinition {
	return gt.values
}
func (gt *Enum) Serialize(value interface{}) interface{} {
	v := value
	rv := reflect.ValueOf(v)
	if kind := rv.Kind(); kind == reflect.Ptr && rv.IsNil() {
		return nil
	} else if kind == reflect.Ptr {
		v = reflect.Indirect(reflect.ValueOf(v)).Interface()
	}
	if enumValue, ok := gt.getValueLookup()[v]; ok {
		return enumValue.Name
	}
	return nil
}
func (gt *Enum) ParseValue(value interface{}) interface{} {
	var v string

	switch value := value.(type) {
	case string:
		v = value
	case *string:
		v = *value
	default:
		return nil
	}
	if enumValue, ok := gt.getNameLookup()[v]; ok {
		return enumValue.Value
	}
	return nil
}
func (gt *Enum) ParseLiteral(valueAST ast.Value) interface{} {
	if valueAST, ok := valueAST.(*ast.EnumValue); ok {
		if enumValue, ok := gt.getNameLookup()[valueAST.Value]; ok {
			return enumValue.Value
		}
	}
	return nil
}
func (gt *Enum) Name() string {
	return gt.PrivateName
}
func (gt *Enum) Description() string {
	return gt.PrivateDescription
}
func (gt *Enum) String() string {
	return gt.PrivateName
}
func (gt *Enum) Error() error {
	return gt.err
}
func (gt *Enum) getValueLookup() map[interface{}]*EnumValueDefinition {
	if len(gt.valuesLookup) > 0 {
		return gt.valuesLookup
	}
	valuesLookup := map[interface{}]*EnumValueDefinition{}
	for _, value := range gt.Values() {
		valuesLookup[value.Value] = value
	}
	gt.valuesLookup = valuesLookup
	return gt.valuesLookup
}

func (gt *Enum) getNameLookup() map[string]*EnumValueDefinition {
	if len(gt.nameLookup) > 0 {
		return gt.nameLookup
	}
	nameLookup := map[string]*EnumValueDefinition{}
	for _, value := range gt.Values() {
		nameLookup[value.Name] = value
	}
	gt.nameLookup = nameLookup
	return gt.nameLookup
}

// InputObject Type Definition
//
// An input object defines a structured collection of fields which may be
// supplied to a field argument.
//
// Using `NonNull` will ensure that a value must be provided by the query
//
// Example:
//
//     var GeoPoint = new InputObject({
//       name: 'GeoPoint',
//       fields: {
//         lat: { type: new NonNull(Float) },
//         lon: { type: new NonNull(Float) },
//         alt: { type: Float, defaultValue: 0 },
//       }
//     });
type InputObject struct {
	PrivateName        string `json:"name"`
	PrivateDescription string `json:"description"`

	typeConfig InputObjectConfig
	fields     InputObjectFieldMap
	init       bool
	err        error
}
type InputObjectFieldConfig struct {
	Type         Input       `json:"type"`
	DefaultValue interface{} `json:"defaultValue"`
	Description  string      `json:"description"`
}
type InputObjectField struct {
	PrivateName        string      `json:"name"`
	Type               Input       `json:"type"`
	DefaultValue       interface{} `json:"defaultValue"`
	PrivateDescription string      `json:"description"`
}

func (st *InputObjectField) Name() string {
	return st.PrivateName
}
func (st *InputObjectField) Description() string {
	return st.PrivateDescription
}
func (st *InputObjectField) String() string {
	return st.PrivateName
}
func (st *InputObjectField) Error() error {
	return nil
}

type InputObjectConfigFieldMap map[string]*InputObjectFieldConfig
type InputObjectFieldMap map[string]*InputObjectField
type InputObjectConfigFieldMapThunk func() InputObjectConfigFieldMap
type InputObjectConfig struct {
	Name        string      `json:"name"`
	Fields      interface{} `json:"fields"`
	Description string      `json:"description"`
}

func NewInputObject(config InputObjectConfig) *InputObject {
	gt := &InputObject{}
	if gt.err = invariant(config.Name != "", "Type must be named."); gt.err != nil {
		return gt
	}

	gt.PrivateName = config.Name
	gt.PrivateDescription = config.Description
	gt.typeConfig = config
	return gt
}

func (gt *InputObject) defineFieldMap() InputObjectFieldMap {
	var (
		fieldMap InputObjectConfigFieldMap
		err      error
	)
	switch fields := gt.typeConfig.Fields.(type) {
	case InputObjectConfigFieldMap:
		fieldMap = fields
	case InputObjectConfigFieldMapThunk:
		fieldMap = fields()
	}
	resultFieldMap := InputObjectFieldMap{}

	if gt.err = invariantf(
		len(fieldMap) > 0,
		`%v fields must be an object with field names as keys or a function which return such an object.`, gt,
	); gt.err != nil {
		return resultFieldMap
	}

	for fieldName, fieldConfig := range fieldMap {
		if fieldConfig == nil {
			continue
		}
		if err = assertValidName(fieldName); err != nil {
			continue
		}
		if gt.err = invariantf(
			fieldConfig.Type != nil,
			`%v.%v field type must be Input Type but got: %v.`, gt, fieldName, fieldConfig.Type,
		); err != nil {
			return resultFieldMap
		}
		field := &InputObjectField{}
		field.PrivateName = fieldName
		field.Type = fieldConfig.Type
		field.PrivateDescription = fieldConfig.Description
		field.DefaultValue = fieldConfig.DefaultValue
		resultFieldMap[fieldName] = field
	}
	gt.init = true
	return resultFieldMap
}

func (gt *InputObject) AddFieldConfig(fieldName string, fieldConfig *InputObjectFieldConfig) {
	if fieldName == "" || fieldConfig == nil {
		return
	}
	fieldMap, ok := gt.typeConfig.Fields.(InputObjectConfigFieldMap)
	if gt.err = invariant(ok, "Cannot add field to a thunk"); gt.err != nil {
		return
	}
	fieldMap[fieldName] = fieldConfig
	gt.fields = gt.defineFieldMap()
}

func (gt *InputObject) Fields() InputObjectFieldMap {
	if !gt.init {
		gt.fields = gt.defineFieldMap()
	}
	return gt.fields
}
func (gt *InputObject) Name() string {
	return gt.PrivateName
}
func (gt *InputObject) Description() string {
	return gt.PrivateDescription
}
func (gt *InputObject) String() string {
	return gt.PrivateName
}
func (gt *InputObject) Error() error {
	return gt.err
}

// List Modifier
//
// A list is a kind of type marker, a wrapping type which points to another
// type. Lists are often created within the context of defining the fields of
// an object type.
//
// Example:
//
//     var PersonType = new Object({
//       name: 'Person',
//       fields: () => ({
//         parents: { type: new List(Person) },
//         children: { type: new List(Person) },
//       })
//     })
//
type List struct {
	OfType Type `json:"ofType"`

	err error
}

func NewList(ofType Type) *List {
	gl := &List{}

	gl.err = invariantf(ofType != nil, `Can only create List of a Type but got: %v.`, ofType)
	if gl.err != nil {
		return gl
	}

	gl.OfType = ofType
	return gl
}
func (gl *List) Name() string {
	return fmt.Sprintf("%v", gl.OfType)
}
func (gl *List) Description() string {
	return ""
}
func (gl *List) String() string {
	if gl.OfType != nil {
		return fmt.Sprintf("[%v]", gl.OfType)
	}
	return ""
}
func (gl *List) Error() error {
	return gl.err
}

// NonNull Modifier
//
// A non-null is a kind of type marker, a wrapping type which points to another
// type. Non-null types enforce that their values are never null and can ensure
// an error is raised if this ever occurs during a request. It is useful for
// fields which you can make a strong guarantee on non-nullability, for example
// usually the id field of a database row will never be null.
//
// Example:
//
//     var RowType = new Object({
//       name: 'Row',
//       fields: () => ({
//         id: { type: new NonNull(String) },
//       })
//     })
//
// Note: the enforcement of non-nullability occurs within the executor.
type NonNull struct {
	OfType Type `json:"ofType"`

	err error
}

func NewNonNull(ofType Type) *NonNull {
	gl := &NonNull{}

	_, isOfTypeNonNull := ofType.(*NonNull)
	gl.err = invariantf(ofType != nil && !isOfTypeNonNull, `Can only create NonNull of a Nullable Type but got: %v.`, ofType)
	if gl.err != nil {
		return gl
	}
	gl.OfType = ofType
	return gl
}
func (gl *NonNull) Name() string {
	return fmt.Sprintf("%v!", gl.OfType)
}
func (gl *NonNull) Description() string {
	return ""
}
func (gl *NonNull) String() string {
	if gl.OfType != nil {
		return gl.Name()
	}
	return ""
}
func (gl *NonNull) Error() error {
	return gl.err
}

var NameRegExp = regexp.MustCompile("^[_a-zA-Z][_a-zA-Z0-9]*$")

func assertValidName(name string) error {
	return invariantf(
		NameRegExp.MatchString(name),
		`Names must match /^[_a-zA-Z][_a-zA-Z0-9]*$/ but "%v" does not.`, name)

}

type ResponsePath struct {
	Prev *ResponsePath
	Key  interface{}
}

// WithKey returns a new responsePath containing the new key.
func (p *ResponsePath) WithKey(key interface{}) *ResponsePath {
	return &ResponsePath{
		Prev: p,
		Key:  key,
	}
}

// AsArray returns an array of path keys.
func (p *ResponsePath) AsArray() []interface{} {
	if p == nil {
		return nil
	}
	return append(p.Prev.AsArray(), p.Key)
}
