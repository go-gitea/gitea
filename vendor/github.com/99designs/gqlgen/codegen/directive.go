package codegen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/ast"
)

type DirectiveList map[string]*Directive

//LocationDirectives filter directives by location
func (dl DirectiveList) LocationDirectives(location string) DirectiveList {
	return locationDirectives(dl, ast.DirectiveLocation(location))
}

type Directive struct {
	*ast.DirectiveDefinition
	Name    string
	Args    []*FieldArgument
	Builtin bool
}

//IsLocation check location directive
func (d *Directive) IsLocation(location ...ast.DirectiveLocation) bool {
	for _, l := range d.Locations {
		for _, a := range location {
			if l == a {
				return true
			}
		}
	}

	return false
}

func locationDirectives(directives DirectiveList, location ...ast.DirectiveLocation) map[string]*Directive {
	mDirectives := make(map[string]*Directive)
	for name, d := range directives {
		if d.IsLocation(location...) {
			mDirectives[name] = d
		}
	}
	return mDirectives
}

func (b *builder) buildDirectives() (map[string]*Directive, error) {
	directives := make(map[string]*Directive, len(b.Schema.Directives))

	for name, dir := range b.Schema.Directives {
		if _, ok := directives[name]; ok {
			return nil, errors.Errorf("directive with name %s already exists", name)
		}

		var args []*FieldArgument
		for _, arg := range dir.Arguments {
			tr, err := b.Binder.TypeReference(arg.Type, nil)
			if err != nil {
				return nil, err
			}

			newArg := &FieldArgument{
				ArgumentDefinition: arg,
				TypeReference:      tr,
				VarName:            templates.ToGoPrivate(arg.Name),
			}

			if arg.DefaultValue != nil {
				var err error
				newArg.Default, err = arg.DefaultValue.Value(nil)
				if err != nil {
					return nil, errors.Errorf("default value for directive argument %s(%s) is not valid: %s", dir.Name, arg.Name, err.Error())
				}
			}
			args = append(args, newArg)
		}

		directives[name] = &Directive{
			DirectiveDefinition: dir,
			Name:                name,
			Args:                args,
			Builtin:             b.Config.Directives[name].SkipRuntime,
		}
	}

	return directives, nil
}

func (b *builder) getDirectives(list ast.DirectiveList) ([]*Directive, error) {
	dirs := make([]*Directive, len(list))
	for i, d := range list {
		argValues := make(map[string]interface{}, len(d.Arguments))
		for _, da := range d.Arguments {
			val, err := da.Value.Value(nil)
			if err != nil {
				return nil, err
			}
			argValues[da.Name] = val
		}
		def, ok := b.Directives[d.Name]
		if !ok {
			return nil, fmt.Errorf("directive %s not found", d.Name)
		}

		var args []*FieldArgument
		for _, a := range def.Args {
			value := a.Default
			if argValue, ok := argValues[a.Name]; ok {
				value = argValue
			}
			args = append(args, &FieldArgument{
				ArgumentDefinition: a.ArgumentDefinition,
				Value:              value,
				VarName:            a.VarName,
				TypeReference:      a.TypeReference,
			})
		}
		dirs[i] = &Directive{
			Name:                d.Name,
			Args:                args,
			DirectiveDefinition: list[i].Definition,
			Builtin:             b.Config.Directives[d.Name].SkipRuntime,
		}

	}

	return dirs, nil
}

func (d *Directive) ArgsFunc() string {
	if len(d.Args) == 0 {
		return ""
	}

	return "dir_" + d.Name + "_args"
}

func (d *Directive) CallArgs() string {
	args := []string{"ctx", "obj", "n"}

	for _, arg := range d.Args {
		args = append(args, "args["+strconv.Quote(arg.Name)+"].("+templates.CurrentImports.LookupType(arg.TypeReference.GO)+")")
	}

	return strings.Join(args, ", ")
}

func (d *Directive) ResolveArgs(obj string, next int) string {
	args := []string{"ctx", obj, fmt.Sprintf("directive%d", next)}

	for _, arg := range d.Args {
		dArg := arg.VarName
		if arg.Value == nil && arg.Default == nil {
			dArg = "nil"
		}

		args = append(args, dArg)
	}

	return strings.Join(args, ", ")
}

func (d *Directive) Declaration() string {
	res := ucFirst(d.Name) + " func(ctx context.Context, obj interface{}, next graphql.Resolver"

	for _, arg := range d.Args {
		res += fmt.Sprintf(", %s %s", templates.ToGoPrivate(arg.Name), templates.CurrentImports.LookupType(arg.TypeReference.GO))
	}

	res += ") (res interface{}, err error)"
	return res
}
