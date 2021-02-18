package codegen

import (
	"fmt"
	"go/types"

	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/99designs/gqlgen/codegen/config"
)

type Interface struct {
	*ast.Definition
	Type         types.Type
	Implementors []InterfaceImplementor
	InTypemap    bool
}

type InterfaceImplementor struct {
	*ast.Definition

	Type    types.Type
	TakeRef bool
}

func (b *builder) buildInterface(typ *ast.Definition) (*Interface, error) {
	obj, err := b.Binder.DefaultUserObject(typ.Name)
	if err != nil {
		panic(err)
	}

	i := &Interface{
		Definition: typ,
		Type:       obj,
		InTypemap:  b.Config.Models.UserDefined(typ.Name),
	}

	interfaceType, err := findGoInterface(i.Type)
	if interfaceType == nil || err != nil {
		return nil, fmt.Errorf("%s is not an interface", i.Type)
	}

	for _, implementor := range b.Schema.GetPossibleTypes(typ) {
		obj, err := b.Binder.DefaultUserObject(implementor.Name)
		if err != nil {
			return nil, fmt.Errorf("%s has no backing go type", implementor.Name)
		}

		implementorType, err := findGoNamedType(obj)
		if err != nil {
			return nil, errors.Wrapf(err, "can not find backing go type %s", obj.String())
		} else if implementorType == nil {
			return nil, fmt.Errorf("can not find backing go type %s", obj.String())
		}

		anyValid := false

		// first check if the value receiver can be nil, eg can we type switch on case Thing:
		if types.Implements(implementorType, interfaceType) {
			i.Implementors = append(i.Implementors, InterfaceImplementor{
				Definition: implementor,
				Type:       obj,
				TakeRef:    !types.IsInterface(obj),
			})
			anyValid = true
		}

		// then check if the pointer receiver can be nil, eg can we type switch on case *Thing:
		if types.Implements(types.NewPointer(implementorType), interfaceType) {
			i.Implementors = append(i.Implementors, InterfaceImplementor{
				Definition: implementor,
				Type:       types.NewPointer(obj),
			})
			anyValid = true
		}

		if !anyValid {
			return nil, fmt.Errorf("%s does not satisfy the interface %s", implementorType.String(), i.Type.String())
		}
	}

	return i, nil
}

func (i *InterfaceImplementor) CanBeNil() bool {
	return config.IsNilable(i.Type)
}
