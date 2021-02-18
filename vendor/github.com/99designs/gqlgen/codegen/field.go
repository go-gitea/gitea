package codegen

import (
	"fmt"
	"go/types"
	"log"
	"reflect"
	"strconv"
	"strings"

	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/ast"
)

type Field struct {
	*ast.FieldDefinition

	TypeReference    *config.TypeReference
	GoFieldType      GoFieldType      // The field type in go, if any
	GoReceiverName   string           // The name of method & var receiver in go, if any
	GoFieldName      string           // The name of the method or var in go, if any
	IsResolver       bool             // Does this field need a resolver
	Args             []*FieldArgument // A list of arguments to be passed to this field
	MethodHasContext bool             // If this is bound to a go method, does the method also take a context
	NoErr            bool             // If this is bound to a go method, does that method have an error as the second argument
	Object           *Object          // A link back to the parent object
	Default          interface{}      // The default value
	Stream           bool             // does this field return a channel?
	Directives       []*Directive
}

func (b *builder) buildField(obj *Object, field *ast.FieldDefinition) (*Field, error) {
	dirs, err := b.getDirectives(field.Directives)
	if err != nil {
		return nil, err
	}

	f := Field{
		FieldDefinition: field,
		Object:          obj,
		Directives:      dirs,
		GoFieldName:     templates.ToGo(field.Name),
		GoFieldType:     GoFieldVariable,
		GoReceiverName:  "obj",
	}

	if field.DefaultValue != nil {
		var err error
		f.Default, err = field.DefaultValue.Value(nil)
		if err != nil {
			return nil, errors.Errorf("default value %s is not valid: %s", field.Name, err.Error())
		}
	}

	for _, arg := range field.Arguments {
		newArg, err := b.buildArg(obj, arg)
		if err != nil {
			return nil, err
		}
		f.Args = append(f.Args, newArg)
	}

	if err = b.bindField(obj, &f); err != nil {
		f.IsResolver = true
		log.Println(err.Error())
	}

	if f.IsResolver && !f.TypeReference.IsPtr() && f.TypeReference.IsStruct() {
		f.TypeReference = b.Binder.PointerTo(f.TypeReference)
	}

	return &f, nil
}

func (b *builder) bindField(obj *Object, f *Field) (errret error) {
	defer func() {
		if f.TypeReference == nil {
			tr, err := b.Binder.TypeReference(f.Type, nil)
			if err != nil {
				errret = err
			}
			f.TypeReference = tr
		}
		if f.TypeReference != nil {
			dirs, err := b.getDirectives(f.TypeReference.Definition.Directives)
			if err != nil {
				errret = err
			}
			f.Directives = append(dirs, f.Directives...)
		}
	}()

	f.Stream = obj.Stream

	switch {
	case f.Name == "__schema":
		f.GoFieldType = GoFieldMethod
		f.GoReceiverName = "ec"
		f.GoFieldName = "introspectSchema"
		return nil
	case f.Name == "__type":
		f.GoFieldType = GoFieldMethod
		f.GoReceiverName = "ec"
		f.GoFieldName = "introspectType"
		return nil
	case f.Name == "_entities":
		f.GoFieldType = GoFieldMethod
		f.GoReceiverName = "ec"
		f.GoFieldName = "__resolve_entities"
		f.MethodHasContext = true
		return nil
	case f.Name == "_service":
		f.GoFieldType = GoFieldMethod
		f.GoReceiverName = "ec"
		f.GoFieldName = "__resolve__service"
		f.MethodHasContext = true
		return nil
	case obj.Root:
		f.IsResolver = true
		return nil
	case b.Config.Models[obj.Name].Fields[f.Name].Resolver:
		f.IsResolver = true
		return nil
	case obj.Type == config.MapType:
		f.GoFieldType = GoFieldMap
		return nil
	case b.Config.Models[obj.Name].Fields[f.Name].FieldName != "":
		f.GoFieldName = b.Config.Models[obj.Name].Fields[f.Name].FieldName
	}

	target, err := b.findBindTarget(obj.Type.(*types.Named), f.GoFieldName)
	if err != nil {
		return err
	}

	pos := b.Binder.ObjectPosition(target)

	switch target := target.(type) {
	case nil:
		objPos := b.Binder.TypePosition(obj.Type)
		return fmt.Errorf(
			"%s:%d adding resolver method for %s.%s, nothing matched",
			objPos.Filename,
			objPos.Line,
			obj.Name,
			f.Name,
		)

	case *types.Func:
		sig := target.Type().(*types.Signature)
		if sig.Results().Len() == 1 {
			f.NoErr = true
		} else if sig.Results().Len() != 2 {
			return fmt.Errorf("method has wrong number of args")
		}
		params := sig.Params()
		// If the first argument is the context, remove it from the comparison and set
		// the MethodHasContext flag so that the context will be passed to this model's method
		if params.Len() > 0 && params.At(0).Type().String() == "context.Context" {
			f.MethodHasContext = true
			vars := make([]*types.Var, params.Len()-1)
			for i := 1; i < params.Len(); i++ {
				vars[i-1] = params.At(i)
			}
			params = types.NewTuple(vars...)
		}

		if err = b.bindArgs(f, params); err != nil {
			return errors.Wrapf(err, "%s:%d", pos.Filename, pos.Line)
		}

		result := sig.Results().At(0)
		tr, err := b.Binder.TypeReference(f.Type, result.Type())
		if err != nil {
			return err
		}

		// success, args and return type match. Bind to method
		f.GoFieldType = GoFieldMethod
		f.GoReceiverName = "obj"
		f.GoFieldName = target.Name()
		f.TypeReference = tr

		return nil

	case *types.Var:
		tr, err := b.Binder.TypeReference(f.Type, target.Type())
		if err != nil {
			return err
		}

		// success, bind to var
		f.GoFieldType = GoFieldVariable
		f.GoReceiverName = "obj"
		f.GoFieldName = target.Name()
		f.TypeReference = tr

		return nil
	default:
		panic(fmt.Errorf("unknown bind target %T for %s", target, f.Name))
	}
}

// findBindTarget attempts to match the name to a field or method on a Type
// with the following priorites:
// 1. Any Fields with a struct tag (see config.StructTag). Errors if more than one match is found
// 2. Any method or field with a matching name. Errors if more than one match is found
// 3. Same logic again for embedded fields
func (b *builder) findBindTarget(t types.Type, name string) (types.Object, error) {
	// NOTE: a struct tag will override both methods and fields
	// Bind to struct tag
	found, err := b.findBindStructTagTarget(t, name)
	if found != nil || err != nil {
		return found, err
	}

	// Search for a method to bind to
	foundMethod, err := b.findBindMethodTarget(t, name)
	if err != nil {
		return nil, err
	}

	// Search for a field to bind to
	foundField, err := b.findBindFieldTarget(t, name)
	if err != nil {
		return nil, err
	}

	switch {
	case foundField == nil && foundMethod != nil:
		// Bind to method
		return foundMethod, nil
	case foundField != nil && foundMethod == nil:
		// Bind to field
		return foundField, nil
	case foundField != nil && foundMethod != nil:
		// Error
		return nil, errors.Errorf("found more than one way to bind for %s", name)
	}

	// Search embeds
	return b.findBindEmbedsTarget(t, name)
}

func (b *builder) findBindStructTagTarget(in types.Type, name string) (types.Object, error) {
	if b.Config.StructTag == "" {
		return nil, nil
	}

	switch t := in.(type) {
	case *types.Named:
		return b.findBindStructTagTarget(t.Underlying(), name)
	case *types.Struct:
		var found types.Object
		for i := 0; i < t.NumFields(); i++ {
			field := t.Field(i)
			if !field.Exported() || field.Embedded() {
				continue
			}
			tags := reflect.StructTag(t.Tag(i))
			if val, ok := tags.Lookup(b.Config.StructTag); ok && equalFieldName(val, name) {
				if found != nil {
					return nil, errors.Errorf("tag %s is ambigious; multiple fields have the same tag value of %s", b.Config.StructTag, val)
				}

				found = field
			}
		}

		return found, nil
	}

	return nil, nil
}

func (b *builder) findBindMethodTarget(in types.Type, name string) (types.Object, error) {
	switch t := in.(type) {
	case *types.Named:
		if _, ok := t.Underlying().(*types.Interface); ok {
			return b.findBindMethodTarget(t.Underlying(), name)
		}

		return b.findBindMethoderTarget(t.Method, t.NumMethods(), name)
	case *types.Interface:
		// FIX-ME: Should use ExplicitMethod here? What's the difference?
		return b.findBindMethoderTarget(t.Method, t.NumMethods(), name)
	}

	return nil, nil
}

func (b *builder) findBindMethoderTarget(methodFunc func(i int) *types.Func, methodCount int, name string) (types.Object, error) {
	var found types.Object
	for i := 0; i < methodCount; i++ {
		method := methodFunc(i)
		if !method.Exported() || !strings.EqualFold(method.Name(), name) {
			continue
		}

		if found != nil {
			return nil, errors.Errorf("found more than one matching method to bind for %s", name)
		}

		found = method
	}

	return found, nil
}

func (b *builder) findBindFieldTarget(in types.Type, name string) (types.Object, error) {
	switch t := in.(type) {
	case *types.Named:
		return b.findBindFieldTarget(t.Underlying(), name)
	case *types.Struct:
		var found types.Object
		for i := 0; i < t.NumFields(); i++ {
			field := t.Field(i)
			if !field.Exported() || !equalFieldName(field.Name(), name) {
				continue
			}

			if found != nil {
				return nil, errors.Errorf("found more than one matching field to bind for %s", name)
			}

			found = field
		}

		return found, nil
	}

	return nil, nil
}

func (b *builder) findBindEmbedsTarget(in types.Type, name string) (types.Object, error) {
	switch t := in.(type) {
	case *types.Named:
		return b.findBindEmbedsTarget(t.Underlying(), name)
	case *types.Struct:
		return b.findBindStructEmbedsTarget(t, name)
	case *types.Interface:
		return b.findBindInterfaceEmbedsTarget(t, name)
	}

	return nil, nil
}

func (b *builder) findBindStructEmbedsTarget(strukt *types.Struct, name string) (types.Object, error) {
	var found types.Object
	for i := 0; i < strukt.NumFields(); i++ {
		field := strukt.Field(i)
		if !field.Embedded() {
			continue
		}

		fieldType := field.Type()
		if ptr, ok := fieldType.(*types.Pointer); ok {
			fieldType = ptr.Elem()
		}

		f, err := b.findBindTarget(fieldType, name)
		if err != nil {
			return nil, err
		}

		if f != nil && found != nil {
			return nil, errors.Errorf("found more than one way to bind for %s", name)
		}

		if f != nil {
			found = f
		}
	}

	return found, nil
}

func (b *builder) findBindInterfaceEmbedsTarget(iface *types.Interface, name string) (types.Object, error) {
	var found types.Object
	for i := 0; i < iface.NumEmbeddeds(); i++ {
		embeddedType := iface.EmbeddedType(i)

		f, err := b.findBindTarget(embeddedType, name)
		if err != nil {
			return nil, err
		}

		if f != nil && found != nil {
			return nil, errors.Errorf("found more than one way to bind for %s", name)
		}

		if f != nil {
			found = f
		}
	}

	return found, nil
}

func (f *Field) HasDirectives() bool {
	return len(f.ImplDirectives()) > 0
}

func (f *Field) DirectiveObjName() string {
	if f.Object.Root {
		return "nil"
	}
	return f.GoReceiverName
}

func (f *Field) ImplDirectives() []*Directive {
	var d []*Directive
	loc := ast.LocationFieldDefinition
	if f.Object.IsInputType() {
		loc = ast.LocationInputFieldDefinition
	}
	for i := range f.Directives {
		if !f.Directives[i].Builtin && f.Directives[i].IsLocation(loc, ast.LocationObject) {
			d = append(d, f.Directives[i])
		}
	}
	return d
}

func (f *Field) IsReserved() bool {
	return strings.HasPrefix(f.Name, "__")
}

func (f *Field) IsMethod() bool {
	return f.GoFieldType == GoFieldMethod
}

func (f *Field) IsVariable() bool {
	return f.GoFieldType == GoFieldVariable
}

func (f *Field) IsMap() bool {
	return f.GoFieldType == GoFieldMap
}

func (f *Field) IsConcurrent() bool {
	if f.Object.DisableConcurrency {
		return false
	}
	return f.MethodHasContext || f.IsResolver
}

func (f *Field) GoNameUnexported() string {
	return templates.ToGoPrivate(f.Name)
}

func (f *Field) ShortInvocation() string {
	return fmt.Sprintf("%s().%s(%s)", f.Object.Definition.Name, f.GoFieldName, f.CallArgs())
}

func (f *Field) ArgsFunc() string {
	if len(f.Args) == 0 {
		return ""
	}

	return "field_" + f.Object.Definition.Name + "_" + f.Name + "_args"
}

func (f *Field) ResolverType() string {
	if !f.IsResolver {
		return ""
	}

	return fmt.Sprintf("%s().%s(%s)", f.Object.Definition.Name, f.GoFieldName, f.CallArgs())
}

func (f *Field) ShortResolverDeclaration() string {
	res := "(ctx context.Context"

	if !f.Object.Root {
		res += fmt.Sprintf(", obj %s", templates.CurrentImports.LookupType(f.Object.Reference()))
	}
	for _, arg := range f.Args {
		res += fmt.Sprintf(", %s %s", arg.VarName, templates.CurrentImports.LookupType(arg.TypeReference.GO))
	}

	result := templates.CurrentImports.LookupType(f.TypeReference.GO)
	if f.Object.Stream {
		result = "<-chan " + result
	}

	res += fmt.Sprintf(") (%s, error)", result)
	return res
}

func (f *Field) ComplexitySignature() string {
	res := "func(childComplexity int"
	for _, arg := range f.Args {
		res += fmt.Sprintf(", %s %s", arg.VarName, templates.CurrentImports.LookupType(arg.TypeReference.GO))
	}
	res += ") int"
	return res
}

func (f *Field) ComplexityArgs() string {
	args := make([]string, len(f.Args))
	for i, arg := range f.Args {
		args[i] = "args[" + strconv.Quote(arg.Name) + "].(" + templates.CurrentImports.LookupType(arg.TypeReference.GO) + ")"
	}

	return strings.Join(args, ", ")
}

func (f *Field) CallArgs() string {
	args := make([]string, 0, len(f.Args)+2)

	if f.IsResolver {
		args = append(args, "rctx")

		if !f.Object.Root {
			args = append(args, "obj")
		}
	} else if f.MethodHasContext {
		args = append(args, "ctx")
	}

	for _, arg := range f.Args {
		args = append(args, "args["+strconv.Quote(arg.Name)+"].("+templates.CurrentImports.LookupType(arg.TypeReference.GO)+")")
	}

	return strings.Join(args, ", ")
}
