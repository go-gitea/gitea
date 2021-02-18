package codegen

import (
	"go/types"
	"strconv"
	"strings"
	"unicode"

	"github.com/99designs/gqlgen/codegen/config"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/ast"
)

type GoFieldType int

const (
	GoFieldUndefined GoFieldType = iota
	GoFieldMethod
	GoFieldVariable
	GoFieldMap
)

type Object struct {
	*ast.Definition

	Type               types.Type
	ResolverInterface  types.Type
	Root               bool
	Fields             []*Field
	Implements         []*ast.Definition
	DisableConcurrency bool
	Stream             bool
	Directives         []*Directive
}

func (b *builder) buildObject(typ *ast.Definition) (*Object, error) {
	dirs, err := b.getDirectives(typ.Directives)
	if err != nil {
		return nil, errors.Wrap(err, typ.Name)
	}

	obj := &Object{
		Definition:         typ,
		Root:               b.Schema.Query == typ || b.Schema.Mutation == typ || b.Schema.Subscription == typ,
		DisableConcurrency: typ == b.Schema.Mutation,
		Stream:             typ == b.Schema.Subscription,
		Directives:         dirs,
		ResolverInterface: types.NewNamed(
			types.NewTypeName(0, b.Config.Exec.Pkg(), typ.Name+"Resolver", nil),
			nil,
			nil,
		),
	}

	if !obj.Root {
		goObject, err := b.Binder.DefaultUserObject(typ.Name)
		if err != nil {
			return nil, err
		}
		obj.Type = goObject
	}

	for _, intf := range b.Schema.GetImplements(typ) {
		obj.Implements = append(obj.Implements, b.Schema.Types[intf.Name])
	}

	for _, field := range typ.Fields {
		if strings.HasPrefix(field.Name, "__") {
			continue
		}

		var f *Field
		f, err = b.buildField(obj, field)
		if err != nil {
			return nil, err
		}

		obj.Fields = append(obj.Fields, f)
	}

	return obj, nil
}

func (o *Object) Reference() types.Type {
	if config.IsNilable(o.Type) {
		return o.Type
	}
	return types.NewPointer(o.Type)
}

type Objects []*Object

func (o *Object) Implementors() string {
	satisfiedBy := strconv.Quote(o.Name)
	for _, s := range o.Implements {
		satisfiedBy += ", " + strconv.Quote(s.Name)
	}
	return "[]string{" + satisfiedBy + "}"
}

func (o *Object) HasResolvers() bool {
	for _, f := range o.Fields {
		if f.IsResolver {
			return true
		}
	}
	return false
}

func (o *Object) HasUnmarshal() bool {
	if o.Type == config.MapType {
		return true
	}
	for i := 0; i < o.Type.(*types.Named).NumMethods(); i++ {
		if o.Type.(*types.Named).Method(i).Name() == "UnmarshalGQL" {
			return true
		}
	}
	return false
}

func (o *Object) HasDirectives() bool {
	if len(o.Directives) > 0 {
		return true
	}
	for _, f := range o.Fields {
		if f.HasDirectives() {
			return true
		}
	}

	return false
}

func (o *Object) IsConcurrent() bool {
	for _, f := range o.Fields {
		if f.IsConcurrent() {
			return true
		}
	}
	return false
}

func (o *Object) IsReserved() bool {
	return strings.HasPrefix(o.Definition.Name, "__")
}

func (o *Object) Description() string {
	return o.Definition.Description
}

func (os Objects) ByName(name string) *Object {
	for i, o := range os {
		if strings.EqualFold(o.Definition.Name, name) {
			return os[i]
		}
	}
	return nil
}

func ucFirst(s string) string {
	if s == "" {
		return ""
	}

	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
