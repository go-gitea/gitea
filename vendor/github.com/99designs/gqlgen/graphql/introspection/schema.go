package introspection

import (
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
)

type Schema struct {
	schema *ast.Schema
}

func (s *Schema) Types() []Type {
	types := make([]Type, 0, len(s.schema.Types))
	for _, typ := range s.schema.Types {
		if strings.HasPrefix(typ.Name, "__") {
			continue
		}
		types = append(types, *WrapTypeFromDef(s.schema, typ))
	}
	return types
}

func (s *Schema) QueryType() *Type {
	return WrapTypeFromDef(s.schema, s.schema.Query)
}

func (s *Schema) MutationType() *Type {
	return WrapTypeFromDef(s.schema, s.schema.Mutation)
}

func (s *Schema) SubscriptionType() *Type {
	return WrapTypeFromDef(s.schema, s.schema.Subscription)
}

func (s *Schema) Directives() []Directive {
	res := make([]Directive, 0, len(s.schema.Directives))

	for _, d := range s.schema.Directives {
		res = append(res, s.directiveFromDef(d))
	}

	return res
}

func (s *Schema) directiveFromDef(d *ast.DirectiveDefinition) Directive {
	locs := make([]string, len(d.Locations))
	for i, loc := range d.Locations {
		locs[i] = string(loc)
	}

	args := make([]InputValue, len(d.Arguments))
	for i, arg := range d.Arguments {
		args[i] = InputValue{
			Name:         arg.Name,
			Description:  arg.Description,
			DefaultValue: defaultValue(arg.DefaultValue),
			Type:         WrapTypeFromType(s.schema, arg.Type),
		}
	}

	return Directive{
		Name:        d.Name,
		Description: d.Description,
		Locations:   locs,
		Args:        args,
	}
}
