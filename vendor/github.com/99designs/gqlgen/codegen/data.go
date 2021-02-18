package codegen

import (
	"fmt"
	"sort"

	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/99designs/gqlgen/codegen/config"
)

// Data is a unified model of the code to be generated. Plugins may modify this structure to do things like implement
// resolvers or directives automatically (eg grpc, validation)
type Data struct {
	Config          *config.Config
	Schema          *ast.Schema
	Directives      DirectiveList
	Objects         Objects
	Inputs          Objects
	Interfaces      map[string]*Interface
	ReferencedTypes map[string]*config.TypeReference
	ComplexityRoots map[string]*Object

	QueryRoot        *Object
	MutationRoot     *Object
	SubscriptionRoot *Object
}

type builder struct {
	Config     *config.Config
	Schema     *ast.Schema
	Binder     *config.Binder
	Directives map[string]*Directive
}

func BuildData(cfg *config.Config) (*Data, error) {
	b := builder{
		Config: cfg,
		Schema: cfg.Schema,
	}

	b.Binder = b.Config.NewBinder()

	var err error
	b.Directives, err = b.buildDirectives()
	if err != nil {
		return nil, err
	}

	dataDirectives := make(map[string]*Directive)
	for name, d := range b.Directives {
		if !d.Builtin {
			dataDirectives[name] = d
		}
	}

	s := Data{
		Config:     cfg,
		Directives: dataDirectives,
		Schema:     b.Schema,
		Interfaces: map[string]*Interface{},
	}

	for _, schemaType := range b.Schema.Types {
		switch schemaType.Kind {
		case ast.Object:
			obj, err := b.buildObject(schemaType)
			if err != nil {
				return nil, errors.Wrap(err, "unable to build object definition")
			}

			s.Objects = append(s.Objects, obj)
		case ast.InputObject:
			input, err := b.buildObject(schemaType)
			if err != nil {
				return nil, errors.Wrap(err, "unable to build input definition")
			}

			s.Inputs = append(s.Inputs, input)

		case ast.Union, ast.Interface:
			s.Interfaces[schemaType.Name], err = b.buildInterface(schemaType)
			if err != nil {
				return nil, errors.Wrap(err, "unable to bind to interface")
			}
		}
	}

	if s.Schema.Query != nil {
		s.QueryRoot = s.Objects.ByName(s.Schema.Query.Name)
	} else {
		return nil, fmt.Errorf("query entry point missing")
	}

	if s.Schema.Mutation != nil {
		s.MutationRoot = s.Objects.ByName(s.Schema.Mutation.Name)
	}

	if s.Schema.Subscription != nil {
		s.SubscriptionRoot = s.Objects.ByName(s.Schema.Subscription.Name)
	}

	if err := b.injectIntrospectionRoots(&s); err != nil {
		return nil, err
	}

	s.ReferencedTypes = b.buildTypes()

	sort.Slice(s.Objects, func(i, j int) bool {
		return s.Objects[i].Definition.Name < s.Objects[j].Definition.Name
	})

	sort.Slice(s.Inputs, func(i, j int) bool {
		return s.Inputs[i].Definition.Name < s.Inputs[j].Definition.Name
	})

	if b.Binder.SawInvalid {
		// if we have a syntax error, show it
		err := cfg.Packages.Errors()
		if len(err) > 0 {
			return nil, err
		}

		// otherwise show a generic error message
		return nil, fmt.Errorf("invalid types were encountered while traversing the go source code, this probably means the invalid code generated isnt correct. add try adding -v to debug")
	}

	return &s, nil
}

func (b *builder) injectIntrospectionRoots(s *Data) error {
	obj := s.Objects.ByName(b.Schema.Query.Name)
	if obj == nil {
		return fmt.Errorf("root query type must be defined")
	}

	__type, err := b.buildField(obj, &ast.FieldDefinition{
		Name: "__type",
		Type: ast.NamedType("__Type", nil),
		Arguments: []*ast.ArgumentDefinition{
			{
				Name: "name",
				Type: ast.NonNullNamedType("String", nil),
			},
		},
	})
	if err != nil {
		return err
	}

	__schema, err := b.buildField(obj, &ast.FieldDefinition{
		Name: "__schema",
		Type: ast.NamedType("__Schema", nil),
	})
	if err != nil {
		return err
	}

	obj.Fields = append(obj.Fields, __type, __schema)

	return nil
}
