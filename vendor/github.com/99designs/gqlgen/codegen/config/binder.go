package config

import (
	"fmt"
	"go/token"
	"go/types"

	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/99designs/gqlgen/internal/code"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/ast"
)

// Binder connects graphql types to golang types using static analysis
type Binder struct {
	pkgs       *code.Packages
	schema     *ast.Schema
	cfg        *Config
	References []*TypeReference
	SawInvalid bool
}

func (c *Config) NewBinder() *Binder {
	return &Binder{
		pkgs:   c.Packages,
		schema: c.Schema,
		cfg:    c,
	}
}

func (b *Binder) TypePosition(typ types.Type) token.Position {
	named, isNamed := typ.(*types.Named)
	if !isNamed {
		return token.Position{
			Filename: "unknown",
		}
	}

	return b.ObjectPosition(named.Obj())
}

func (b *Binder) ObjectPosition(typ types.Object) token.Position {
	if typ == nil {
		return token.Position{
			Filename: "unknown",
		}
	}
	pkg := b.pkgs.Load(typ.Pkg().Path())
	return pkg.Fset.Position(typ.Pos())
}

func (b *Binder) FindTypeFromName(name string) (types.Type, error) {
	pkgName, typeName := code.PkgAndType(name)
	return b.FindType(pkgName, typeName)
}

func (b *Binder) FindType(pkgName string, typeName string) (types.Type, error) {
	if pkgName == "" {
		if typeName == "map[string]interface{}" {
			return MapType, nil
		}

		if typeName == "interface{}" {
			return InterfaceType, nil
		}
	}

	obj, err := b.FindObject(pkgName, typeName)
	if err != nil {
		return nil, err
	}

	if fun, isFunc := obj.(*types.Func); isFunc {
		return fun.Type().(*types.Signature).Params().At(0).Type(), nil
	}
	return obj.Type(), nil
}

var MapType = types.NewMap(types.Typ[types.String], types.NewInterfaceType(nil, nil).Complete())
var InterfaceType = types.NewInterfaceType(nil, nil)

func (b *Binder) DefaultUserObject(name string) (types.Type, error) {
	models := b.cfg.Models[name].Model
	if len(models) == 0 {
		return nil, fmt.Errorf(name + " not found in typemap")
	}

	if models[0] == "map[string]interface{}" {
		return MapType, nil
	}

	if models[0] == "interface{}" {
		return InterfaceType, nil
	}

	pkgName, typeName := code.PkgAndType(models[0])
	if pkgName == "" {
		return nil, fmt.Errorf("missing package name for %s", name)
	}

	obj, err := b.FindObject(pkgName, typeName)
	if err != nil {
		return nil, err
	}

	return obj.Type(), nil
}

func (b *Binder) FindObject(pkgName string, typeName string) (types.Object, error) {
	if pkgName == "" {
		return nil, fmt.Errorf("package cannot be nil")
	}
	fullName := typeName
	if pkgName != "" {
		fullName = pkgName + "." + typeName
	}

	pkg := b.pkgs.LoadWithTypes(pkgName)
	if pkg == nil {
		return nil, errors.Errorf("required package was not loaded: %s", fullName)
	}

	// function based marshalers take precedence
	for astNode, def := range pkg.TypesInfo.Defs {
		// only look at defs in the top scope
		if def == nil || def.Parent() == nil || def.Parent() != pkg.Types.Scope() {
			continue
		}

		if astNode.Name == "Marshal"+typeName {
			return def, nil
		}
	}

	// then look for types directly
	for astNode, def := range pkg.TypesInfo.Defs {
		// only look at defs in the top scope
		if def == nil || def.Parent() == nil || def.Parent() != pkg.Types.Scope() {
			continue
		}

		if astNode.Name == typeName {
			return def, nil
		}
	}

	return nil, errors.Errorf("unable to find type %s\n", fullName)
}

func (b *Binder) PointerTo(ref *TypeReference) *TypeReference {
	newRef := &TypeReference{
		GO:          types.NewPointer(ref.GO),
		GQL:         ref.GQL,
		CastType:    ref.CastType,
		Definition:  ref.Definition,
		Unmarshaler: ref.Unmarshaler,
		Marshaler:   ref.Marshaler,
		IsMarshaler: ref.IsMarshaler,
	}

	b.References = append(b.References, newRef)
	return newRef
}

// TypeReference is used by args and field types. The Definition can refer to both input and output types.
type TypeReference struct {
	Definition  *ast.Definition
	GQL         *ast.Type
	GO          types.Type  // Type of the field being bound. Could be a pointer or a value type of Target.
	Target      types.Type  // The actual type that we know how to bind to. May require pointer juggling when traversing to fields.
	CastType    types.Type  // Before calling marshalling functions cast from/to this base type
	Marshaler   *types.Func // When using external marshalling functions this will point to the Marshal function
	Unmarshaler *types.Func // When using external marshalling functions this will point to the Unmarshal function
	IsMarshaler bool        // Does the type implement graphql.Marshaler and graphql.Unmarshaler
}

func (ref *TypeReference) Elem() *TypeReference {
	if p, isPtr := ref.GO.(*types.Pointer); isPtr {
		return &TypeReference{
			GO:          p.Elem(),
			Target:      ref.Target,
			GQL:         ref.GQL,
			CastType:    ref.CastType,
			Definition:  ref.Definition,
			Unmarshaler: ref.Unmarshaler,
			Marshaler:   ref.Marshaler,
			IsMarshaler: ref.IsMarshaler,
		}
	}

	if ref.IsSlice() {
		return &TypeReference{
			GO:          ref.GO.(*types.Slice).Elem(),
			Target:      ref.Target,
			GQL:         ref.GQL.Elem,
			CastType:    ref.CastType,
			Definition:  ref.Definition,
			Unmarshaler: ref.Unmarshaler,
			Marshaler:   ref.Marshaler,
			IsMarshaler: ref.IsMarshaler,
		}
	}
	return nil
}

func (t *TypeReference) IsPtr() bool {
	_, isPtr := t.GO.(*types.Pointer)
	return isPtr
}

func (t *TypeReference) IsNilable() bool {
	return IsNilable(t.GO)
}

func (t *TypeReference) IsSlice() bool {
	_, isSlice := t.GO.(*types.Slice)
	return t.GQL.Elem != nil && isSlice
}

func (t *TypeReference) IsNamed() bool {
	_, isSlice := t.GO.(*types.Named)
	return isSlice
}

func (t *TypeReference) IsStruct() bool {
	_, isStruct := t.GO.Underlying().(*types.Struct)
	return isStruct
}

func (t *TypeReference) IsScalar() bool {
	return t.Definition.Kind == ast.Scalar
}

func (t *TypeReference) UniquenessKey() string {
	var nullability = "O"
	if t.GQL.NonNull {
		nullability = "N"
	}

	var elemNullability = ""
	if t.GQL.Elem != nil && t.GQL.Elem.NonNull {
		// Fix for #896
		elemNullability = "ᚄ"
	}
	return nullability + t.Definition.Name + "2" + templates.TypeIdentifier(t.GO) + elemNullability
}

func (t *TypeReference) MarshalFunc() string {
	if t.Definition == nil {
		panic(errors.New("Definition missing for " + t.GQL.Name()))
	}

	if t.Definition.Kind == ast.InputObject {
		return ""
	}

	return "marshal" + t.UniquenessKey()
}

func (t *TypeReference) UnmarshalFunc() string {
	if t.Definition == nil {
		panic(errors.New("Definition missing for " + t.GQL.Name()))
	}

	if !t.Definition.IsInputType() {
		return ""
	}

	return "unmarshal" + t.UniquenessKey()
}

func (t *TypeReference) IsTargetNilable() bool {
	return IsNilable(t.Target)
}

func (b *Binder) PushRef(ret *TypeReference) {
	b.References = append(b.References, ret)
}

func isMap(t types.Type) bool {
	if t == nil {
		return true
	}
	_, ok := t.(*types.Map)
	return ok
}

func isIntf(t types.Type) bool {
	if t == nil {
		return true
	}
	_, ok := t.(*types.Interface)
	return ok
}

func (b *Binder) TypeReference(schemaType *ast.Type, bindTarget types.Type) (ret *TypeReference, err error) {
	if !isValid(bindTarget) {
		b.SawInvalid = true
		return nil, fmt.Errorf("%s has an invalid type", schemaType.Name())
	}

	var pkgName, typeName string
	def := b.schema.Types[schemaType.Name()]
	defer func() {
		if err == nil && ret != nil {
			b.PushRef(ret)
		}
	}()

	if len(b.cfg.Models[schemaType.Name()].Model) == 0 {
		return nil, fmt.Errorf("%s was not found", schemaType.Name())
	}

	for _, model := range b.cfg.Models[schemaType.Name()].Model {
		if model == "map[string]interface{}" {
			if !isMap(bindTarget) {
				continue
			}
			return &TypeReference{
				Definition: def,
				GQL:        schemaType,
				GO:         MapType,
			}, nil
		}

		if model == "interface{}" {
			if !isIntf(bindTarget) {
				continue
			}
			return &TypeReference{
				Definition: def,
				GQL:        schemaType,
				GO:         InterfaceType,
			}, nil
		}

		pkgName, typeName = code.PkgAndType(model)
		if pkgName == "" {
			return nil, fmt.Errorf("missing package name for %s", schemaType.Name())
		}

		ref := &TypeReference{
			Definition: def,
			GQL:        schemaType,
		}

		obj, err := b.FindObject(pkgName, typeName)
		if err != nil {
			return nil, err
		}

		if fun, isFunc := obj.(*types.Func); isFunc {
			ref.GO = fun.Type().(*types.Signature).Params().At(0).Type()
			ref.Marshaler = fun
			ref.Unmarshaler = types.NewFunc(0, fun.Pkg(), "Unmarshal"+typeName, nil)
		} else if hasMethod(obj.Type(), "MarshalGQL") && hasMethod(obj.Type(), "UnmarshalGQL") {
			ref.GO = obj.Type()
			ref.IsMarshaler = true
		} else if underlying := basicUnderlying(obj.Type()); def.IsLeafType() && underlying != nil && underlying.Kind() == types.String {
			// TODO delete before v1. Backwards compatibility case for named types wrapping strings (see #595)

			ref.GO = obj.Type()
			ref.CastType = underlying

			underlyingRef, err := b.TypeReference(&ast.Type{NamedType: "String"}, nil)
			if err != nil {
				return nil, err
			}

			ref.Marshaler = underlyingRef.Marshaler
			ref.Unmarshaler = underlyingRef.Unmarshaler
		} else {
			ref.GO = obj.Type()
		}

		ref.Target = ref.GO
		ref.GO = b.CopyModifiersFromAst(schemaType, ref.GO)

		if bindTarget != nil {
			if err = code.CompatibleTypes(ref.GO, bindTarget); err != nil {
				continue
			}
			ref.GO = bindTarget
		}

		return ref, nil
	}

	return nil, fmt.Errorf("%s is incompatible with %s", schemaType.Name(), bindTarget.String())
}

func isValid(t types.Type) bool {
	basic, isBasic := t.(*types.Basic)
	if !isBasic {
		return true
	}
	return basic.Kind() != types.Invalid
}

func (b *Binder) CopyModifiersFromAst(t *ast.Type, base types.Type) types.Type {
	if t.Elem != nil {
		child := b.CopyModifiersFromAst(t.Elem, base)
		if _, isStruct := child.Underlying().(*types.Struct); isStruct && !b.cfg.OmitSliceElementPointers {
			child = types.NewPointer(child)
		}
		return types.NewSlice(child)
	}

	var isInterface bool
	if named, ok := base.(*types.Named); ok {
		_, isInterface = named.Underlying().(*types.Interface)
	}

	if !isInterface && !IsNilable(base) && !t.NonNull {
		return types.NewPointer(base)
	}

	return base
}

func IsNilable(t types.Type) bool {
	if namedType, isNamed := t.(*types.Named); isNamed {
		return IsNilable(namedType.Underlying())
	}
	_, isPtr := t.(*types.Pointer)
	_, isMap := t.(*types.Map)
	_, isInterface := t.(*types.Interface)
	_, isSlice := t.(*types.Slice)
	_, isChan := t.(*types.Chan)
	return isPtr || isMap || isInterface || isSlice || isChan
}

func hasMethod(it types.Type, name string) bool {
	if ptr, isPtr := it.(*types.Pointer); isPtr {
		it = ptr.Elem()
	}
	namedType, ok := it.(*types.Named)
	if !ok {
		return false
	}

	for i := 0; i < namedType.NumMethods(); i++ {
		if namedType.Method(i).Name() == name {
			return true
		}
	}
	return false
}

func basicUnderlying(it types.Type) *types.Basic {
	if ptr, isPtr := it.(*types.Pointer); isPtr {
		it = ptr.Elem()
	}
	namedType, ok := it.(*types.Named)
	if !ok {
		return nil
	}

	if basic, ok := namedType.Underlying().(*types.Basic); ok {
		return basic
	}

	return nil
}
