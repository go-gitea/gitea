package graphql

import (
	"fmt"
)

type SchemaConfig struct {
	Query        *Object
	Mutation     *Object
	Subscription *Object
	Types        []Type
	Directives   []*Directive
	Extensions   []Extension
}

type TypeMap map[string]Type

// Schema Definition
// A Schema is created by supplying the root types of each type of operation,
// query, mutation (optional) and subscription (optional). A schema definition is then supplied to the
// validator and executor.
// Example:
//     myAppSchema, err := NewSchema(SchemaConfig({
//       Query: MyAppQueryRootType,
//       Mutation: MyAppMutationRootType,
//       Subscription: MyAppSubscriptionRootType,
//     });
// Note: If an array of `directives` are provided to GraphQLSchema, that will be
// the exact list of directives represented and allowed. If `directives` is not
// provided then a default set of the specified directives (e.g. @include and
// @skip) will be used. If you wish to provide *additional* directives to these
// specified directives, you must explicitly declare them. Example:
//
//     const MyAppSchema = new GraphQLSchema({
//       ...
//       directives: specifiedDirectives.concat([ myCustomDirective ]),
//     })
type Schema struct {
	typeMap    TypeMap
	directives []*Directive

	queryType        *Object
	mutationType     *Object
	subscriptionType *Object
	implementations  map[string][]*Object
	possibleTypeMap  map[string]map[string]bool
	extensions       []Extension
}

func NewSchema(config SchemaConfig) (Schema, error) {
	var err error

	schema := Schema{}

	if err = invariant(config.Query != nil, "Schema query must be Object Type but got: nil."); err != nil {
		return schema, err
	}

	// if schema config contains error at creation time, return those errors
	if config.Query != nil && config.Query.err != nil {
		return schema, config.Query.err
	}
	if config.Mutation != nil && config.Mutation.err != nil {
		return schema, config.Mutation.err
	}

	schema.queryType = config.Query
	schema.mutationType = config.Mutation
	schema.subscriptionType = config.Subscription

	// Provide specified directives (e.g. @include and @skip) by default.
	schema.directives = config.Directives
	if len(schema.directives) == 0 {
		schema.directives = SpecifiedDirectives
	}
	// Ensure directive definitions are error-free
	for _, dir := range schema.directives {
		if dir.err != nil {
			return schema, dir.err
		}
	}

	// Build type map now to detect any errors within this schema.
	typeMap := TypeMap{}
	initialTypes := []Type{}
	if schema.QueryType() != nil {
		initialTypes = append(initialTypes, schema.QueryType())
	}
	if schema.MutationType() != nil {
		initialTypes = append(initialTypes, schema.MutationType())
	}
	if schema.SubscriptionType() != nil {
		initialTypes = append(initialTypes, schema.SubscriptionType())
	}
	if SchemaType != nil {
		initialTypes = append(initialTypes, SchemaType)
	}

	for _, ttype := range config.Types {
		// assume that user will never add a nil object to config
		initialTypes = append(initialTypes, ttype)
	}

	for _, ttype := range initialTypes {
		if ttype.Error() != nil {
			return schema, ttype.Error()
		}
		if typeMap, err = typeMapReducer(&schema, typeMap, ttype); err != nil {
			return schema, err
		}
	}

	schema.typeMap = typeMap

	// Keep track of all implementations by interface name.
	if schema.implementations == nil {
		schema.implementations = map[string][]*Object{}
	}
	for _, ttype := range schema.typeMap {
		if ttype, ok := ttype.(*Object); ok {
			for _, iface := range ttype.Interfaces() {
				impls, ok := schema.implementations[iface.Name()]
				if impls == nil || !ok {
					impls = []*Object{}
				}
				impls = append(impls, ttype)
				schema.implementations[iface.Name()] = impls
			}
		}
	}

	// Enforce correct interface implementations
	for _, ttype := range schema.typeMap {
		if ttype, ok := ttype.(*Object); ok {
			for _, iface := range ttype.Interfaces() {
				err := assertObjectImplementsInterface(&schema, ttype, iface)
				if err != nil {
					return schema, err
				}
			}
		}
	}

	// Add extensions from config
	if len(config.Extensions) != 0 {
		schema.extensions = config.Extensions
	}

	return schema, nil
}

//Added Check implementation of interfaces at runtime..
//Add Implementations at Runtime..
func (gq *Schema) AddImplementation() error {

	// Keep track of all implementations by interface name.
	if gq.implementations == nil {
		gq.implementations = map[string][]*Object{}
	}
	for _, ttype := range gq.typeMap {
		if ttype, ok := ttype.(*Object); ok {
			for _, iface := range ttype.Interfaces() {
				impls, ok := gq.implementations[iface.Name()]
				if impls == nil || !ok {
					impls = []*Object{}
				}
				impls = append(impls, ttype)
				gq.implementations[iface.Name()] = impls
			}
		}
	}

	// Enforce correct interface implementations
	for _, ttype := range gq.typeMap {
		if ttype, ok := ttype.(*Object); ok {
			for _, iface := range ttype.Interfaces() {
				err := assertObjectImplementsInterface(gq, ttype, iface)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

//Edited. To check add Types at RunTime..
//Append Runtime schema to typeMap
func (gq *Schema) AppendType(objectType Type) error {
	if objectType.Error() != nil {
		return objectType.Error()
	}
	var err error
	gq.typeMap, err = typeMapReducer(gq, gq.typeMap, objectType)
	if err != nil {
		return err
	}
	//Now Add interface implementation..
	return gq.AddImplementation()
}

func (gq *Schema) QueryType() *Object {
	return gq.queryType
}

func (gq *Schema) MutationType() *Object {
	return gq.mutationType
}

func (gq *Schema) SubscriptionType() *Object {
	return gq.subscriptionType
}

func (gq *Schema) Directives() []*Directive {
	return gq.directives
}

func (gq *Schema) Directive(name string) *Directive {
	for _, directive := range gq.Directives() {
		if directive.Name == name {
			return directive
		}
	}
	return nil
}

func (gq *Schema) TypeMap() TypeMap {
	return gq.typeMap
}

func (gq *Schema) Type(name string) Type {
	return gq.TypeMap()[name]
}

func (gq *Schema) PossibleTypes(abstractType Abstract) []*Object {
	switch abstractType := abstractType.(type) {
	case *Union:
		return abstractType.Types()
	case *Interface:
		if impls, ok := gq.implementations[abstractType.Name()]; ok {
			return impls
		}
	}
	return []*Object{}
}
func (gq *Schema) IsPossibleType(abstractType Abstract, possibleType *Object) bool {
	possibleTypeMap := gq.possibleTypeMap
	if possibleTypeMap == nil {
		possibleTypeMap = map[string]map[string]bool{}
	}

	if typeMap, ok := possibleTypeMap[abstractType.Name()]; !ok {
		typeMap = map[string]bool{}
		for _, possibleType := range gq.PossibleTypes(abstractType) {
			typeMap[possibleType.Name()] = true
		}
		possibleTypeMap[abstractType.Name()] = typeMap
	}

	gq.possibleTypeMap = possibleTypeMap
	if typeMap, ok := possibleTypeMap[abstractType.Name()]; ok {
		isPossible, _ := typeMap[possibleType.Name()]
		return isPossible
	}
	return false
}

// AddExtensions can be used to add additional extensions to the schema
func (gq *Schema) AddExtensions(e ...Extension) {
	gq.extensions = append(gq.extensions, e...)
}

// map-reduce
func typeMapReducer(schema *Schema, typeMap TypeMap, objectType Type) (TypeMap, error) {
	var err error
	if objectType == nil || objectType.Name() == "" {
		return typeMap, nil
	}

	// first:
	switch objectType := objectType.(type) {
	case *List:
		if objectType.OfType != nil {
			return typeMapReducer(schema, typeMap, objectType.OfType)
		}
	case *NonNull:
		if objectType.OfType != nil {
			return typeMapReducer(schema, typeMap, objectType.OfType)
		}
	case *Object:
		if objectType.err != nil {
			return typeMap, objectType.err
		}
	}

	if mappedObjectType, ok := typeMap[objectType.Name()]; ok {
		err = invariantf(
			mappedObjectType == objectType,
			`Schema must contain unique named types but contains multiple types named "%v".`, objectType.Name())
		return typeMap, err
	}
	typeMap[objectType.Name()] = objectType

	// second:
	switch objectType := objectType.(type) {
	case *Union, *Interface:
		types := schema.PossibleTypes(objectType)
		if objectType.Error() != nil {
			return typeMap, objectType.Error()
		}
		for _, innerObjectType := range types {
			if innerObjectType.err != nil {
				return typeMap, innerObjectType.err
			}
			if typeMap, err = typeMapReducer(schema, typeMap, innerObjectType); err != nil {
				return typeMap, err
			}
		}
	case *Object:
		interfaces := objectType.Interfaces()
		if objectType.err != nil {
			return typeMap, objectType.err
		}
		for _, innerObjectType := range interfaces {
			if innerObjectType.err != nil {
				return typeMap, innerObjectType.err
			}
			if typeMap, err = typeMapReducer(schema, typeMap, innerObjectType); err != nil {
				return typeMap, err
			}
		}
	}

	switch objectType := objectType.(type) {
	case *Object:
		fieldMap := objectType.Fields()
		if objectType.err != nil {
			return typeMap, objectType.err
		}
		for _, field := range fieldMap {
			for _, arg := range field.Args {
				typeMap, err = typeMapReducer(schema, typeMap, arg.Type)
				if err != nil {
					return typeMap, err
				}
			}
			typeMap, err = typeMapReducer(schema, typeMap, field.Type)
			if err != nil {
				return typeMap, err
			}
		}
	case *Interface:
		fieldMap := objectType.Fields()
		if objectType.err != nil {
			return typeMap, objectType.err
		}
		for _, field := range fieldMap {
			for _, arg := range field.Args {
				typeMap, err = typeMapReducer(schema, typeMap, arg.Type)
				if err != nil {
					return typeMap, err
				}
			}
			typeMap, err = typeMapReducer(schema, typeMap, field.Type)
			if err != nil {
				return typeMap, err
			}
		}
	case *InputObject:
		fieldMap := objectType.Fields()
		if objectType.err != nil {
			return typeMap, objectType.err
		}
		for _, field := range fieldMap {
			typeMap, err = typeMapReducer(schema, typeMap, field.Type)
			if err != nil {
				return typeMap, err
			}
		}
	}
	return typeMap, nil
}

func assertObjectImplementsInterface(schema *Schema, object *Object, iface *Interface) error {
	objectFieldMap := object.Fields()
	ifaceFieldMap := iface.Fields()

	// Assert each interface field is implemented.
	for fieldName := range ifaceFieldMap {
		objectField := objectFieldMap[fieldName]
		ifaceField := ifaceFieldMap[fieldName]

		// Assert interface field exists on object.
		err := invariantf(
			objectField != nil,
			`"%v" expects field "%v" but "%v" does not `+
				`provide it.`, iface, fieldName, object)

		if err != nil {
			return err
		}

		// Assert interface field type is satisfied by object field type, by being
		// a valid subtype. (covariant)
		err = invariant(
			isTypeSubTypeOf(schema, objectField.Type, ifaceField.Type),
			fmt.Sprintf(`%v.%v expects type "%v" but `+
				`%v.%v provides type "%v".`,
				iface, fieldName, ifaceField.Type,
				object, fieldName, objectField.Type),
		)
		if err != nil {
			return err
		}

		// Assert each interface field arg is implemented.
		for _, ifaceArg := range ifaceField.Args {
			argName := ifaceArg.PrivateName
			var objectArg *Argument
			for _, arg := range objectField.Args {
				if arg.PrivateName == argName {
					objectArg = arg
					break
				}
			}
			// Assert interface field arg exists on object field.
			err = invariant(
				objectArg != nil,
				fmt.Sprintf(`%v.%v expects argument "%v" but `+
					`%v.%v does not provide it.`,
					iface, fieldName, argName,
					object, fieldName),
			)
			if err != nil {
				return err
			}

			// Assert interface field arg type matches object field arg type.
			// (invariant)
			err = invariant(
				isEqualType(ifaceArg.Type, objectArg.Type),
				fmt.Sprintf(
					`%v.%v(%v:) expects type "%v" `+
						`but %v.%v(%v:) provides `+
						`type "%v".`,
					iface, fieldName, argName, ifaceArg.Type,
					object, fieldName, argName, objectArg.Type),
			)
			if err != nil {
				return err
			}
		}
		// Assert additional arguments must not be required.
		for _, objectArg := range objectField.Args {
			argName := objectArg.PrivateName
			var ifaceArg *Argument
			for _, arg := range ifaceField.Args {
				if arg.PrivateName == argName {
					ifaceArg = arg
					break
				}
			}

			if ifaceArg == nil {
				_, ok := objectArg.Type.(*NonNull)
				err = invariant(
					!ok,
					fmt.Sprintf(`%v.%v(%v:) is of required type `+
						`"%v" but is not also provided by the interface %v.%v.`,
						object, fieldName, argName,
						objectArg.Type, iface, fieldName),
				)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func isEqualType(typeA Type, typeB Type) bool {
	// Equivalent type is a valid subtype
	if typeA == typeB {
		return true
	}
	// If either type is non-null, the other must also be non-null.
	if typeA, ok := typeA.(*NonNull); ok {
		if typeB, ok := typeB.(*NonNull); ok {
			return isEqualType(typeA.OfType, typeB.OfType)
		}
	}
	// If either type is a list, the other must also be a list.
	if typeA, ok := typeA.(*List); ok {
		if typeB, ok := typeB.(*List); ok {
			return isEqualType(typeA.OfType, typeB.OfType)
		}
	}
	// Otherwise the types are not equal.
	return false
}

// isTypeSubTypeOf Provided a type and a super type, return true if the first type is either
// equal or a subset of the second super type (covariant).
func isTypeSubTypeOf(schema *Schema, maybeSubType Type, superType Type) bool {
	// Equivalent type is a valid subtype
	if maybeSubType == superType {
		return true
	}

	// If superType is non-null, maybeSubType must also be nullable.
	if superType, ok := superType.(*NonNull); ok {
		if maybeSubType, ok := maybeSubType.(*NonNull); ok {
			return isTypeSubTypeOf(schema, maybeSubType.OfType, superType.OfType)
		}
		return false
	}
	if maybeSubType, ok := maybeSubType.(*NonNull); ok {
		// If superType is nullable, maybeSubType may be non-null.
		return isTypeSubTypeOf(schema, maybeSubType.OfType, superType)
	}

	// If superType type is a list, maybeSubType type must also be a list.
	if superType, ok := superType.(*List); ok {
		if maybeSubType, ok := maybeSubType.(*List); ok {
			return isTypeSubTypeOf(schema, maybeSubType.OfType, superType.OfType)
		}
		return false
	} else if _, ok := maybeSubType.(*List); ok {
		// If superType is not a list, maybeSubType must also be not a list.
		return false
	}

	// If superType type is an abstract type, maybeSubType type may be a currently
	// possible object type.
	if superType, ok := superType.(*Interface); ok {
		if maybeSubType, ok := maybeSubType.(*Object); ok && schema.IsPossibleType(superType, maybeSubType) {
			return true
		}
	}
	if superType, ok := superType.(*Union); ok {
		if maybeSubType, ok := maybeSubType.(*Object); ok && schema.IsPossibleType(superType, maybeSubType) {
			return true
		}
	}

	// Otherwise, the child type is not a valid subtype of the parent type.
	return false
}
