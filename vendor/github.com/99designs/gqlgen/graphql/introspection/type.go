package introspection

import (
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
)

type Type struct {
	schema *ast.Schema
	def    *ast.Definition
	typ    *ast.Type
}

func WrapTypeFromDef(s *ast.Schema, def *ast.Definition) *Type {
	if def == nil {
		return nil
	}
	return &Type{schema: s, def: def}
}

func WrapTypeFromType(s *ast.Schema, typ *ast.Type) *Type {
	if typ == nil {
		return nil
	}

	if !typ.NonNull && typ.NamedType != "" {
		return &Type{schema: s, def: s.Types[typ.NamedType]}
	}
	return &Type{schema: s, typ: typ}
}

func (t *Type) Kind() string {
	if t.typ != nil {
		if t.typ.NonNull {
			return "NON_NULL"
		}

		if t.typ.Elem != nil {
			return "LIST"
		}
	} else {
		return string(t.def.Kind)
	}

	panic("UNKNOWN")
}

func (t *Type) Name() *string {
	if t.def == nil {
		return nil
	}
	return &t.def.Name
}

func (t *Type) Description() string {
	if t.def == nil {
		return ""
	}
	return t.def.Description
}

func (t *Type) Fields(includeDeprecated bool) []Field {
	if t.def == nil || (t.def.Kind != ast.Object && t.def.Kind != ast.Interface) {
		return []Field{}
	}
	fields := []Field{}
	for _, f := range t.def.Fields {
		if strings.HasPrefix(f.Name, "__") {
			continue
		}

		if !includeDeprecated && f.Directives.ForName("deprecated") != nil {
			continue
		}

		var args []InputValue
		for _, arg := range f.Arguments {
			args = append(args, InputValue{
				Type:         WrapTypeFromType(t.schema, arg.Type),
				Name:         arg.Name,
				Description:  arg.Description,
				DefaultValue: defaultValue(arg.DefaultValue),
			})
		}

		fields = append(fields, Field{
			Name:        f.Name,
			Description: f.Description,
			Args:        args,
			Type:        WrapTypeFromType(t.schema, f.Type),
			deprecation: f.Directives.ForName("deprecated"),
		})
	}
	return fields
}

func (t *Type) InputFields() []InputValue {
	if t.def == nil || t.def.Kind != ast.InputObject {
		return []InputValue{}
	}

	res := []InputValue{}
	for _, f := range t.def.Fields {
		res = append(res, InputValue{
			Name:         f.Name,
			Description:  f.Description,
			Type:         WrapTypeFromType(t.schema, f.Type),
			DefaultValue: defaultValue(f.DefaultValue),
		})
	}
	return res
}

func defaultValue(value *ast.Value) *string {
	if value == nil {
		return nil
	}
	val := value.String()
	return &val
}

func (t *Type) Interfaces() []Type {
	if t.def == nil || t.def.Kind != ast.Object {
		return []Type{}
	}

	res := []Type{}
	for _, intf := range t.def.Interfaces {
		res = append(res, *WrapTypeFromDef(t.schema, t.schema.Types[intf]))
	}

	return res
}

func (t *Type) PossibleTypes() []Type {
	if t.def == nil || (t.def.Kind != ast.Interface && t.def.Kind != ast.Union) {
		return []Type{}
	}

	res := []Type{}
	for _, pt := range t.schema.GetPossibleTypes(t.def) {
		res = append(res, *WrapTypeFromDef(t.schema, pt))
	}
	return res
}

func (t *Type) EnumValues(includeDeprecated bool) []EnumValue {
	if t.def == nil || t.def.Kind != ast.Enum {
		return []EnumValue{}
	}

	res := []EnumValue{}
	for _, val := range t.def.EnumValues {
		if !includeDeprecated && val.Directives.ForName("deprecated") != nil {
			continue
		}

		res = append(res, EnumValue{
			Name:        val.Name,
			Description: val.Description,
			deprecation: val.Directives.ForName("deprecated"),
		})
	}
	return res
}

func (t *Type) OfType() *Type {
	if t.typ == nil {
		return nil
	}
	if t.typ.NonNull {
		// fake non null nodes
		cpy := *t.typ
		cpy.NonNull = false

		return WrapTypeFromType(t.schema, &cpy)
	}
	return WrapTypeFromType(t.schema, t.typ.Elem)
}
