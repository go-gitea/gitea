package federation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/99designs/gqlgen/plugin"
)

type federation struct {
	Entities []*Entity
}

// New returns a federation plugin that injects
// federated directives and types into the schema
func New() plugin.Plugin {
	return &federation{}
}

// Name returns the plugin name
func (f *federation) Name() string {
	return "federation"
}

// MutateConfig mutates the configuration
func (f *federation) MutateConfig(cfg *config.Config) error {
	builtins := config.TypeMap{
		"_Service": {
			Model: config.StringList{
				"github.com/99designs/gqlgen/plugin/federation/fedruntime.Service",
			},
		},
		"_Entity": {
			Model: config.StringList{
				"github.com/99designs/gqlgen/plugin/federation/fedruntime.Entity",
			},
		},
		"Entity": {
			Model: config.StringList{
				"github.com/99designs/gqlgen/plugin/federation/fedruntime.Entity",
			},
		},
		"_Any": {
			Model: config.StringList{"github.com/99designs/gqlgen/graphql.Map"},
		},
	}
	for typeName, entry := range builtins {
		if cfg.Models.Exists(typeName) {
			return fmt.Errorf("%v already exists which must be reserved when Federation is enabled", typeName)
		}
		cfg.Models[typeName] = entry
	}
	cfg.Directives["external"] = config.DirectiveConfig{SkipRuntime: true}
	cfg.Directives["requires"] = config.DirectiveConfig{SkipRuntime: true}
	cfg.Directives["provides"] = config.DirectiveConfig{SkipRuntime: true}
	cfg.Directives["key"] = config.DirectiveConfig{SkipRuntime: true}
	cfg.Directives["extends"] = config.DirectiveConfig{SkipRuntime: true}

	return nil
}

func (f *federation) InjectSourceEarly() *ast.Source {
	return &ast.Source{
		Name: "federation/directives.graphql",
		Input: `
scalar _Any
scalar _FieldSet

directive @external on FIELD_DEFINITION
directive @requires(fields: _FieldSet!) on FIELD_DEFINITION
directive @provides(fields: _FieldSet!) on FIELD_DEFINITION
directive @key(fields: _FieldSet!) on OBJECT | INTERFACE
directive @extends on OBJECT
`,
		BuiltIn: true,
	}
}

// InjectSources creates a GraphQL Entity type with all
// the fields that had the @key directive
func (f *federation) InjectSourceLate(schema *ast.Schema) *ast.Source {
	f.setEntities(schema)

	entities := ""
	resolvers := ""
	for i, e := range f.Entities {
		if i != 0 {
			entities += " | "
		}
		entities += e.Name

		if e.ResolverName != "" {
			resolverArgs := ""
			for _, field := range e.KeyFields {
				resolverArgs += fmt.Sprintf("%s: %s,", field.Field.Name, field.Field.Type.String())
			}
			resolvers += fmt.Sprintf("\t%s(%s): %s!\n", e.ResolverName, resolverArgs, e.Def.Name)
		}

	}

	if len(f.Entities) == 0 {
		// It's unusual for a service not to have any entities, but
		// possible if it only exports top-level queries and mutations.
		return nil
	}

	// resolvers can be empty if a service defines only "empty
	// extend" types.  This should be rare.
	if resolvers != "" {
		resolvers = `
# fake type to build resolver interfaces for users to implement
type Entity {
	` + resolvers + `
}
`
	}

	return &ast.Source{
		Name:    "federation/entity.graphql",
		BuiltIn: true,
		Input: `
# a union of all types that use the @key directive
union _Entity = ` + entities + `
` + resolvers + `
type _Service {
  sdl: String
}

extend type Query {
  _entities(representations: [_Any!]!): [_Entity]!
  _service: _Service!
}
`,
	}
}

// Entity represents a federated type
// that was declared in the GQL schema.
type Entity struct {
	Name         string      // The same name as the type declaration
	KeyFields    []*KeyField // The fields declared in @key.
	ResolverName string      // The resolver name, such as FindUserByID
	Def          *ast.Definition
	Requires     []*Requires
}

type KeyField struct {
	Field         *ast.FieldDefinition
	TypeReference *config.TypeReference // The Go representation of that field type
}

// Requires represents an @requires clause
type Requires struct {
	Name   string          // the name of the field
	Fields []*RequireField // the name of the sibling fields
}

// RequireField is similar to an entity but it is a field not
// an object
type RequireField struct {
	Name          string                // The same name as the type declaration
	NameGo        string                // The Go struct field name
	TypeReference *config.TypeReference // The Go representation of that field type
}

func (e *Entity) allFieldsAreExternal() bool {
	for _, field := range e.Def.Fields {
		if field.Directives.ForName("external") == nil {
			return false
		}
	}
	return true
}

func (f *federation) GenerateCode(data *codegen.Data) error {
	if len(f.Entities) > 0 {
		if data.Objects.ByName("Entity") != nil {
			data.Objects.ByName("Entity").Root = true
		}
		for _, e := range f.Entities {
			obj := data.Objects.ByName(e.Def.Name)
			for _, field := range obj.Fields {
				// Storing key fields in a slice rather than a map
				// to preserve insertion order at the tradeoff of higher
				// lookup complexity.
				keyField := f.getKeyField(e.KeyFields, field.Name)
				if keyField != nil {
					keyField.TypeReference = field.TypeReference
				}
				for _, r := range e.Requires {
					for _, rf := range r.Fields {
						if rf.Name == field.Name {
							rf.TypeReference = field.TypeReference
							rf.NameGo = field.GoFieldName
						}
					}
				}
			}
		}
	}

	return templates.Render(templates.Options{
		PackageName:     data.Config.Federation.Package,
		Filename:        data.Config.Federation.Filename,
		Data:            f,
		GeneratedHeader: true,
		Packages:        data.Config.Packages,
	})
}

func (f *federation) getKeyField(keyFields []*KeyField, fieldName string) *KeyField {
	for _, field := range keyFields {
		if field.Field.Name == fieldName {
			return field
		}
	}
	return nil
}

func (f *federation) setEntities(schema *ast.Schema) {
	for _, schemaType := range schema.Types {
		if schemaType.Kind == ast.Object {
			dir := schemaType.Directives.ForName("key") // TODO: interfaces
			if dir != nil {
				if len(dir.Arguments) > 1 {
					panic("Multiple arguments are not currently supported in @key declaration.")
				}
				fieldName := dir.Arguments[0].Value.Raw // TODO: multiple arguments
				if strings.Contains(fieldName, "{") {
					panic("Nested fields are not currently supported in @key declaration.")
				}

				requires := []*Requires{}
				for _, f := range schemaType.Fields {
					dir := f.Directives.ForName("requires")
					if dir == nil {
						continue
					}
					fields := strings.Split(dir.Arguments[0].Value.Raw, " ")
					requireFields := []*RequireField{}
					for _, f := range fields {
						requireFields = append(requireFields, &RequireField{
							Name: f,
						})
					}
					requires = append(requires, &Requires{
						Name:   f.Name,
						Fields: requireFields,
					})
				}

				fieldNames := strings.Split(fieldName, " ")
				keyFields := make([]*KeyField, len(fieldNames))
				resolverName := fmt.Sprintf("find%sBy", schemaType.Name)
				for i, f := range fieldNames {
					field := schemaType.Fields.ForName(f)

					keyFields[i] = &KeyField{Field: field}
					if i > 0 {
						resolverName += "And"
					}
					resolverName += templates.ToGo(f)

				}

				e := &Entity{
					Name:         schemaType.Name,
					KeyFields:    keyFields,
					Def:          schemaType,
					ResolverName: resolverName,
					Requires:     requires,
				}
				// If our schema has a field with a type defined in
				// another service, then we need to define an "empty
				// extend" of that type in this service, so this service
				// knows what the type is like.  But the graphql-server
				// will never ask us to actually resolve this "empty
				// extend", so we don't require a resolver function for
				// it.  (Well, it will never ask in practice; it's
				// unclear whether the spec guarantees this.  See
				// https://github.com/apollographql/apollo-server/issues/3852
				// ).  Example:
				//    type MyType {
				//       myvar: TypeDefinedInOtherService
				//    }
				//    // Federation needs this type, but
				//    // it doesn't need a resolver for it!
				//    extend TypeDefinedInOtherService @key(fields: "id") {
				//       id: ID @external
				//    }
				if e.allFieldsAreExternal() {
					e.ResolverName = ""
				}

				f.Entities = append(f.Entities, e)
			}
		}
	}

	// make sure order remains stable across multiple builds
	sort.Slice(f.Entities, func(i, j int) bool {
		return f.Entities[i].Name < f.Entities[j].Name
	})
}
