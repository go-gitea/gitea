package graphql

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/printer"
)

const (
	TypeKindScalar      = "SCALAR"
	TypeKindObject      = "OBJECT"
	TypeKindInterface   = "INTERFACE"
	TypeKindUnion       = "UNION"
	TypeKindEnum        = "ENUM"
	TypeKindInputObject = "INPUT_OBJECT"
	TypeKindList        = "LIST"
	TypeKindNonNull     = "NON_NULL"
)

// SchemaType is type definition for __Schema
var SchemaType *Object

// DirectiveType is type definition for __Directive
var DirectiveType *Object

// TypeType is type definition for __Type
var TypeType *Object

// FieldType is type definition for __Field
var FieldType *Object

// InputValueType is type definition for __InputValue
var InputValueType *Object

// EnumValueType is type definition for __EnumValue
var EnumValueType *Object

// TypeKindEnumType is type definition for __TypeKind
var TypeKindEnumType *Enum

// DirectiveLocationEnumType is type definition for __DirectiveLocation
var DirectiveLocationEnumType *Enum

// Meta-field definitions.

// SchemaMetaFieldDef Meta field definition for Schema
var SchemaMetaFieldDef *FieldDefinition

// TypeMetaFieldDef Meta field definition for types
var TypeMetaFieldDef *FieldDefinition

// TypeNameMetaFieldDef Meta field definition for type names
var TypeNameMetaFieldDef *FieldDefinition

func init() {

	TypeKindEnumType = NewEnum(EnumConfig{
		Name:        "__TypeKind",
		Description: "An enum describing what kind of type a given `__Type` is",
		Values: EnumValueConfigMap{
			"SCALAR": &EnumValueConfig{
				Value:       TypeKindScalar,
				Description: "Indicates this type is a scalar.",
			},
			"OBJECT": &EnumValueConfig{
				Value: TypeKindObject,
				Description: "Indicates this type is an object. " +
					"`fields` and `interfaces` are valid fields.",
			},
			"INTERFACE": &EnumValueConfig{
				Value: TypeKindInterface,
				Description: "Indicates this type is an interface. " +
					"`fields` and `possibleTypes` are valid fields.",
			},
			"UNION": &EnumValueConfig{
				Value: TypeKindUnion,
				Description: "Indicates this type is a union. " +
					"`possibleTypes` is a valid field.",
			},
			"ENUM": &EnumValueConfig{
				Value: TypeKindEnum,
				Description: "Indicates this type is an enum. " +
					"`enumValues` is a valid field.",
			},
			"INPUT_OBJECT": &EnumValueConfig{
				Value: TypeKindInputObject,
				Description: "Indicates this type is an input object. " +
					"`inputFields` is a valid field.",
			},
			"LIST": &EnumValueConfig{
				Value: TypeKindList,
				Description: "Indicates this type is a list. " +
					"`ofType` is a valid field.",
			},
			"NON_NULL": &EnumValueConfig{
				Value: TypeKindNonNull,
				Description: "Indicates this type is a non-null. " +
					"`ofType` is a valid field.",
			},
		},
	})

	DirectiveLocationEnumType = NewEnum(EnumConfig{
		Name: "__DirectiveLocation",
		Description: "A Directive can be adjacent to many parts of the GraphQL language, a " +
			"__DirectiveLocation describes one such possible adjacencies.",
		Values: EnumValueConfigMap{
			"QUERY": &EnumValueConfig{
				Value:       DirectiveLocationQuery,
				Description: "Location adjacent to a query operation.",
			},
			"MUTATION": &EnumValueConfig{
				Value:       DirectiveLocationMutation,
				Description: "Location adjacent to a mutation operation.",
			},
			"SUBSCRIPTION": &EnumValueConfig{
				Value:       DirectiveLocationSubscription,
				Description: "Location adjacent to a subscription operation.",
			},
			"FIELD": &EnumValueConfig{
				Value:       DirectiveLocationField,
				Description: "Location adjacent to a field.",
			},
			"FRAGMENT_DEFINITION": &EnumValueConfig{
				Value:       DirectiveLocationFragmentDefinition,
				Description: "Location adjacent to a fragment definition.",
			},
			"FRAGMENT_SPREAD": &EnumValueConfig{
				Value:       DirectiveLocationFragmentSpread,
				Description: "Location adjacent to a fragment spread.",
			},
			"INLINE_FRAGMENT": &EnumValueConfig{
				Value:       DirectiveLocationInlineFragment,
				Description: "Location adjacent to an inline fragment.",
			},
			"SCHEMA": &EnumValueConfig{
				Value:       DirectiveLocationSchema,
				Description: "Location adjacent to a schema definition.",
			},
			"SCALAR": &EnumValueConfig{
				Value:       DirectiveLocationScalar,
				Description: "Location adjacent to a scalar definition.",
			},
			"OBJECT": &EnumValueConfig{
				Value:       DirectiveLocationObject,
				Description: "Location adjacent to a object definition.",
			},
			"FIELD_DEFINITION": &EnumValueConfig{
				Value:       DirectiveLocationFieldDefinition,
				Description: "Location adjacent to a field definition.",
			},
			"ARGUMENT_DEFINITION": &EnumValueConfig{
				Value:       DirectiveLocationArgumentDefinition,
				Description: "Location adjacent to an argument definition.",
			},
			"INTERFACE": &EnumValueConfig{
				Value:       DirectiveLocationInterface,
				Description: "Location adjacent to an interface definition.",
			},
			"UNION": &EnumValueConfig{
				Value:       DirectiveLocationUnion,
				Description: "Location adjacent to a union definition.",
			},
			"ENUM": &EnumValueConfig{
				Value:       DirectiveLocationEnum,
				Description: "Location adjacent to an enum definition.",
			},
			"ENUM_VALUE": &EnumValueConfig{
				Value:       DirectiveLocationEnumValue,
				Description: "Location adjacent to an enum value definition.",
			},
			"INPUT_OBJECT": &EnumValueConfig{
				Value:       DirectiveLocationInputObject,
				Description: "Location adjacent to an input object type definition.",
			},
			"INPUT_FIELD_DEFINITION": &EnumValueConfig{
				Value:       DirectiveLocationInputFieldDefinition,
				Description: "Location adjacent to an input object field definition.",
			},
		},
	})

	// Note: some fields (for e.g "fields", "interfaces") are defined later due to cyclic reference
	TypeType = NewObject(ObjectConfig{
		Name: "__Type",
		Description: "The fundamental unit of any GraphQL Schema is the type. There are " +
			"many kinds of types in GraphQL as represented by the `__TypeKind` enum." +
			"\n\nDepending on the kind of a type, certain fields describe " +
			"information about that type. Scalar types provide no information " +
			"beyond a name and description, while Enum types provide their values. " +
			"Object and Interface types provide the fields they describe. Abstract " +
			"types, Union and Interface, provide the Object types possible " +
			"at runtime. List and NonNull types compose other types.",

		Fields: Fields{
			"kind": &Field{
				Type: NewNonNull(TypeKindEnumType),
				Resolve: func(p ResolveParams) (interface{}, error) {
					switch p.Source.(type) {
					case *Scalar:
						return TypeKindScalar, nil
					case *Object:
						return TypeKindObject, nil
					case *Interface:
						return TypeKindInterface, nil
					case *Union:
						return TypeKindUnion, nil
					case *Enum:
						return TypeKindEnum, nil
					case *InputObject:
						return TypeKindInputObject, nil
					case *List:
						return TypeKindList, nil
					case *NonNull:
						return TypeKindNonNull, nil
					}
					return nil, fmt.Errorf("Unknown kind of type: %v", p.Source)
				},
			},
			"name": &Field{
				Type: String,
			},
			"description": &Field{
				Type: String,
			},
			"fields":        &Field{},
			"interfaces":    &Field{},
			"possibleTypes": &Field{},
			"enumValues":    &Field{},
			"inputFields":   &Field{},
			"ofType":        &Field{},
		},
	})

	InputValueType = NewObject(ObjectConfig{
		Name: "__InputValue",
		Description: "Arguments provided to Fields or Directives and the input fields of an " +
			"InputObject are represented as Input Values which describe their type " +
			"and optionally a default value.",
		Fields: Fields{
			"name": &Field{
				Type: NewNonNull(String),
			},
			"description": &Field{
				Type: String,
			},
			"type": &Field{
				Type: NewNonNull(TypeType),
			},
			"defaultValue": &Field{
				Type: String,
				Description: "A GraphQL-formatted string representing the default value for this " +
					"input value.",
				Resolve: func(p ResolveParams) (interface{}, error) {
					if inputVal, ok := p.Source.(*Argument); ok {
						if inputVal.DefaultValue == nil {
							return nil, nil
						}
						if isNullish(inputVal.DefaultValue) {
							return nil, nil
						}
						astVal := astFromValue(inputVal.DefaultValue, inputVal)
						return printer.Print(astVal), nil
					}
					if inputVal, ok := p.Source.(*InputObjectField); ok {
						if inputVal.DefaultValue == nil {
							return nil, nil
						}
						astVal := astFromValue(inputVal.DefaultValue, inputVal)
						return printer.Print(astVal), nil
					}
					return nil, nil
				},
			},
		},
	})

	FieldType = NewObject(ObjectConfig{
		Name: "__Field",
		Description: "Object and Interface types are described by a list of Fields, each of " +
			"which has a name, potentially a list of arguments, and a return type.",
		Fields: Fields{
			"name": &Field{
				Type: NewNonNull(String),
			},
			"description": &Field{
				Type: String,
			},
			"args": &Field{
				Type: NewNonNull(NewList(NewNonNull(InputValueType))),
				Resolve: func(p ResolveParams) (interface{}, error) {
					if field, ok := p.Source.(*FieldDefinition); ok {
						return field.Args, nil
					}
					return []interface{}{}, nil
				},
			},
			"type": &Field{
				Type: NewNonNull(TypeType),
			},
			"isDeprecated": &Field{
				Type: NewNonNull(Boolean),
				Resolve: func(p ResolveParams) (interface{}, error) {
					if field, ok := p.Source.(*FieldDefinition); ok {
						return (field.DeprecationReason != ""), nil
					}
					return false, nil
				},
			},
			"deprecationReason": &Field{
				Type: String,
				Resolve: func(p ResolveParams) (interface{}, error) {
					if field, ok := p.Source.(*FieldDefinition); ok {
						if field.DeprecationReason != "" {
							return field.DeprecationReason, nil
						}
					}
					return nil, nil
				},
			},
		},
	})

	DirectiveType = NewObject(ObjectConfig{
		Name: "__Directive",
		Description: "A Directive provides a way to describe alternate runtime execution and " +
			"type validation behavior in a GraphQL document. " +
			"\n\nIn some cases, you need to provide options to alter GraphQL's " +
			"execution behavior in ways field arguments will not suffice, such as " +
			"conditionally including or skipping a field. Directives provide this by " +
			"describing additional information to the executor.",
		Fields: Fields{
			"name": &Field{
				Type: NewNonNull(String),
			},
			"description": &Field{
				Type: String,
			},
			"locations": &Field{
				Type: NewNonNull(NewList(
					NewNonNull(DirectiveLocationEnumType),
				)),
			},
			"args": &Field{
				Type: NewNonNull(NewList(
					NewNonNull(InputValueType),
				)),
			},
			// NOTE: the following three fields are deprecated and are no longer part
			// of the GraphQL specification.
			"onOperation": &Field{
				DeprecationReason: "Use `locations`.",
				Type:              NewNonNull(Boolean),
				Resolve: func(p ResolveParams) (interface{}, error) {
					if dir, ok := p.Source.(*Directive); ok {
						res := false
						for _, loc := range dir.Locations {
							if loc == DirectiveLocationQuery ||
								loc == DirectiveLocationMutation ||
								loc == DirectiveLocationSubscription {
								res = true
								break
							}
						}
						return res, nil
					}
					return false, nil
				},
			},
			"onFragment": &Field{
				DeprecationReason: "Use `locations`.",
				Type:              NewNonNull(Boolean),
				Resolve: func(p ResolveParams) (interface{}, error) {
					if dir, ok := p.Source.(*Directive); ok {
						res := false
						for _, loc := range dir.Locations {
							if loc == DirectiveLocationFragmentSpread ||
								loc == DirectiveLocationInlineFragment ||
								loc == DirectiveLocationFragmentDefinition {
								res = true
								break
							}
						}
						return res, nil
					}
					return false, nil
				},
			},
			"onField": &Field{
				DeprecationReason: "Use `locations`.",
				Type:              NewNonNull(Boolean),
				Resolve: func(p ResolveParams) (interface{}, error) {
					if dir, ok := p.Source.(*Directive); ok {
						res := false
						for _, loc := range dir.Locations {
							if loc == DirectiveLocationField {
								res = true
								break
							}
						}
						return res, nil
					}
					return false, nil
				},
			},
		},
	})

	SchemaType = NewObject(ObjectConfig{
		Name: "__Schema",
		Description: `A GraphQL Schema defines the capabilities of a GraphQL server. ` +
			`It exposes all available types and directives on the server, as well as ` +
			`the entry points for query, mutation, and subscription operations.`,
		Fields: Fields{
			"types": &Field{
				Description: "A list of all types supported by this server.",
				Type: NewNonNull(NewList(
					NewNonNull(TypeType),
				)),
				Resolve: func(p ResolveParams) (interface{}, error) {
					if schema, ok := p.Source.(Schema); ok {
						results := []Type{}
						for _, ttype := range schema.TypeMap() {
							results = append(results, ttype)
						}
						return results, nil
					}
					return []Type{}, nil
				},
			},
			"queryType": &Field{
				Description: "The type that query operations will be rooted at.",
				Type:        NewNonNull(TypeType),
				Resolve: func(p ResolveParams) (interface{}, error) {
					if schema, ok := p.Source.(Schema); ok {
						return schema.QueryType(), nil
					}
					return nil, nil
				},
			},
			"mutationType": &Field{
				Description: `If this server supports mutation, the type that ` +
					`mutation operations will be rooted at.`,
				Type: TypeType,
				Resolve: func(p ResolveParams) (interface{}, error) {
					if schema, ok := p.Source.(Schema); ok {
						if schema.MutationType() != nil {
							return schema.MutationType(), nil
						}
					}
					return nil, nil
				},
			},
			"subscriptionType": &Field{
				Description: `If this server supports subscription, the type that ` +
					`subscription operations will be rooted at.`,
				Type: TypeType,
				Resolve: func(p ResolveParams) (interface{}, error) {
					if schema, ok := p.Source.(Schema); ok {
						if schema.SubscriptionType() != nil {
							return schema.SubscriptionType(), nil
						}
					}
					return nil, nil
				},
			},
			"directives": &Field{
				Description: `A list of all directives supported by this server.`,
				Type: NewNonNull(NewList(
					NewNonNull(DirectiveType),
				)),
				Resolve: func(p ResolveParams) (interface{}, error) {
					if schema, ok := p.Source.(Schema); ok {
						return schema.Directives(), nil
					}
					return nil, nil
				},
			},
		},
	})

	EnumValueType = NewObject(ObjectConfig{
		Name: "__EnumValue",
		Description: "One possible value for a given Enum. Enum values are unique values, not " +
			"a placeholder for a string or numeric value. However an Enum value is " +
			"returned in a JSON response as a string.",
		Fields: Fields{
			"name": &Field{
				Type: NewNonNull(String),
			},
			"description": &Field{
				Type: String,
			},
			"isDeprecated": &Field{
				Type: NewNonNull(Boolean),
				Resolve: func(p ResolveParams) (interface{}, error) {
					if field, ok := p.Source.(*EnumValueDefinition); ok {
						return (field.DeprecationReason != ""), nil
					}
					return false, nil
				},
			},
			"deprecationReason": &Field{
				Type: String,
				Resolve: func(p ResolveParams) (interface{}, error) {
					if field, ok := p.Source.(*EnumValueDefinition); ok {
						if field.DeprecationReason != "" {
							return field.DeprecationReason, nil
						}
					}
					return nil, nil
				},
			},
		},
	})

	// Again, adding field configs to __Type that have cyclic reference here
	// because golang don't like them too much during init/compile-time
	TypeType.AddFieldConfig("fields", &Field{
		Type: NewList(NewNonNull(FieldType)),
		Args: FieldConfigArgument{
			"includeDeprecated": &ArgumentConfig{
				Type:         Boolean,
				DefaultValue: false,
			},
		},
		Resolve: func(p ResolveParams) (interface{}, error) {
			includeDeprecated, _ := p.Args["includeDeprecated"].(bool)
			switch ttype := p.Source.(type) {
			case *Object:
				if ttype == nil {
					return nil, nil
				}
				fields := []*FieldDefinition{}
				var fieldNames sort.StringSlice
				for name, field := range ttype.Fields() {
					if !includeDeprecated && field.DeprecationReason != "" {
						continue
					}
					fieldNames = append(fieldNames, name)
				}
				sort.Sort(fieldNames)
				for _, name := range fieldNames {
					fields = append(fields, ttype.Fields()[name])
				}
				return fields, nil
			case *Interface:
				if ttype == nil {
					return nil, nil
				}
				fields := []*FieldDefinition{}
				for _, field := range ttype.Fields() {
					if !includeDeprecated && field.DeprecationReason != "" {
						continue
					}
					fields = append(fields, field)
				}
				return fields, nil
			}
			return nil, nil
		},
	})
	TypeType.AddFieldConfig("interfaces", &Field{
		Type: NewList(NewNonNull(TypeType)),
		Resolve: func(p ResolveParams) (interface{}, error) {
			if ttype, ok := p.Source.(*Object); ok {
				return ttype.Interfaces(), nil
			}
			return nil, nil
		},
	})
	TypeType.AddFieldConfig("possibleTypes", &Field{
		Type: NewList(NewNonNull(TypeType)),
		Resolve: func(p ResolveParams) (interface{}, error) {
			switch ttype := p.Source.(type) {
			case *Interface:
				return p.Info.Schema.PossibleTypes(ttype), nil
			case *Union:
				return p.Info.Schema.PossibleTypes(ttype), nil
			}
			return nil, nil
		},
	})
	TypeType.AddFieldConfig("enumValues", &Field{
		Type: NewList(NewNonNull(EnumValueType)),
		Args: FieldConfigArgument{
			"includeDeprecated": &ArgumentConfig{
				Type:         Boolean,
				DefaultValue: false,
			},
		},
		Resolve: func(p ResolveParams) (interface{}, error) {
			includeDeprecated, _ := p.Args["includeDeprecated"].(bool)
			if ttype, ok := p.Source.(*Enum); ok {
				if includeDeprecated {
					return ttype.Values(), nil
				}
				values := []*EnumValueDefinition{}
				for _, value := range ttype.Values() {
					if value.DeprecationReason != "" {
						continue
					}
					values = append(values, value)
				}
				return values, nil
			}
			return nil, nil
		},
	})
	TypeType.AddFieldConfig("inputFields", &Field{
		Type: NewList(NewNonNull(InputValueType)),
		Resolve: func(p ResolveParams) (interface{}, error) {
			if ttype, ok := p.Source.(*InputObject); ok {
				fields := []*InputObjectField{}
				for _, field := range ttype.Fields() {
					fields = append(fields, field)
				}
				return fields, nil
			}
			return nil, nil
		},
	})
	TypeType.AddFieldConfig("ofType", &Field{
		Type: TypeType,
	})

	SchemaType.ensureCache()
	DirectiveType.ensureCache()
	TypeType.ensureCache()
	FieldType.ensureCache()
	InputValueType.ensureCache()
	EnumValueType.ensureCache()

	// Note that these are FieldDefinition and not FieldConfig,
	// so the format for args is different.
	SchemaMetaFieldDef = &FieldDefinition{
		Name:        "__schema",
		Type:        NewNonNull(SchemaType),
		Description: "Access the current type schema of this server.",
		Args:        []*Argument{},
		Resolve: func(p ResolveParams) (interface{}, error) {
			return p.Info.Schema, nil
		},
	}
	TypeMetaFieldDef = &FieldDefinition{
		Name:        "__type",
		Type:        TypeType,
		Description: "Request the type information of a single type.",
		Args: []*Argument{
			{
				PrivateName: "name",
				Type:        NewNonNull(String),
			},
		},
		Resolve: func(p ResolveParams) (interface{}, error) {
			name, ok := p.Args["name"].(string)
			if !ok {
				return nil, nil
			}
			return p.Info.Schema.Type(name), nil
		},
	}

	TypeNameMetaFieldDef = &FieldDefinition{
		Name:        "__typename",
		Type:        NewNonNull(String),
		Description: "The name of the current Object type at runtime.",
		Args:        []*Argument{},
		Resolve: func(p ResolveParams) (interface{}, error) {
			return p.Info.ParentType.Name(), nil
		},
	}

}

// Produces a GraphQL Value AST given a Golang value.
//
// Optionally, a GraphQL type may be provided, which will be used to
// disambiguate between value primitives.
//
// | JSON Value    | GraphQL Value        |
// | ------------- | -------------------- |
// | Object        | Input Object         |
// | Array         | List                 |
// | Boolean       | Boolean              |
// | String        | String / Enum Value  |
// | Number        | Int / Float          |

func astFromValue(value interface{}, ttype Type) ast.Value {

	if ttype, ok := ttype.(*NonNull); ok {
		// Note: we're not checking that the result is non-null.
		// This function is not responsible for validating the input value.
		val := astFromValue(value, ttype.OfType)
		return val
	}
	if isNullish(value) {
		return nil
	}
	valueVal := reflect.ValueOf(value)
	if !valueVal.IsValid() {
		return nil
	}
	if valueVal.Type().Kind() == reflect.Ptr {
		valueVal = valueVal.Elem()
	}
	if !valueVal.IsValid() {
		return nil
	}

	// Convert Golang slice to GraphQL list. If the Type is a list, but
	// the value is not an array, convert the value using the list's item type.
	if ttype, ok := ttype.(*List); ok {
		if valueVal.Type().Kind() == reflect.Slice {
			itemType := ttype.OfType
			values := []ast.Value{}
			for i := 0; i < valueVal.Len(); i++ {
				item := valueVal.Index(i).Interface()
				itemAST := astFromValue(item, itemType)
				if itemAST != nil {
					values = append(values, itemAST)
				}
			}
			return ast.NewListValue(&ast.ListValue{
				Values: values,
			})
		}
		// Because GraphQL will accept single values as a "list of one" when
		// expecting a list, if there's a non-array value and an expected list type,
		// create an AST using the list's item type.
		val := astFromValue(value, ttype.OfType)
		return val
	}

	if valueVal.Type().Kind() == reflect.Map {
		// TODO: implement astFromValue from Map to Value
	}

	if value, ok := value.(bool); ok {
		return ast.NewBooleanValue(&ast.BooleanValue{
			Value: value,
		})
	}
	if value, ok := value.(int); ok {
		if ttype == Float {
			return ast.NewIntValue(&ast.IntValue{
				Value: fmt.Sprintf("%v.0", value),
			})
		}
		return ast.NewIntValue(&ast.IntValue{
			Value: fmt.Sprintf("%v", value),
		})
	}
	if value, ok := value.(float32); ok {
		return ast.NewFloatValue(&ast.FloatValue{
			Value: fmt.Sprintf("%v", value),
		})
	}
	if value, ok := value.(float64); ok {
		return ast.NewFloatValue(&ast.FloatValue{
			Value: fmt.Sprintf("%v", value),
		})
	}

	if value, ok := value.(string); ok {
		if _, ok := ttype.(*Enum); ok {
			return ast.NewEnumValue(&ast.EnumValue{
				Value: fmt.Sprintf("%v", value),
			})
		}
		return ast.NewStringValue(&ast.StringValue{
			Value: fmt.Sprintf("%v", value),
		})
	}

	// fallback, treat as string
	return ast.NewStringValue(&ast.StringValue{
		Value: fmt.Sprintf("%v", value),
	})
}
