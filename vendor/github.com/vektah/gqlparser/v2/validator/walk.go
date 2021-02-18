package validator

import (
	"context"
	"fmt"

	"github.com/vektah/gqlparser/v2/ast"
)

type Events struct {
	operationVisitor []func(walker *Walker, operation *ast.OperationDefinition)
	field            []func(walker *Walker, field *ast.Field)
	fragment         []func(walker *Walker, fragment *ast.FragmentDefinition)
	inlineFragment   []func(walker *Walker, inlineFragment *ast.InlineFragment)
	fragmentSpread   []func(walker *Walker, fragmentSpread *ast.FragmentSpread)
	directive        []func(walker *Walker, directive *ast.Directive)
	directiveList    []func(walker *Walker, directives []*ast.Directive)
	value            []func(walker *Walker, value *ast.Value)
}

func (o *Events) OnOperation(f func(walker *Walker, operation *ast.OperationDefinition)) {
	o.operationVisitor = append(o.operationVisitor, f)
}
func (o *Events) OnField(f func(walker *Walker, field *ast.Field)) {
	o.field = append(o.field, f)
}
func (o *Events) OnFragment(f func(walker *Walker, fragment *ast.FragmentDefinition)) {
	o.fragment = append(o.fragment, f)
}
func (o *Events) OnInlineFragment(f func(walker *Walker, inlineFragment *ast.InlineFragment)) {
	o.inlineFragment = append(o.inlineFragment, f)
}
func (o *Events) OnFragmentSpread(f func(walker *Walker, fragmentSpread *ast.FragmentSpread)) {
	o.fragmentSpread = append(o.fragmentSpread, f)
}
func (o *Events) OnDirective(f func(walker *Walker, directive *ast.Directive)) {
	o.directive = append(o.directive, f)
}
func (o *Events) OnDirectiveList(f func(walker *Walker, directives []*ast.Directive)) {
	o.directiveList = append(o.directiveList, f)
}
func (o *Events) OnValue(f func(walker *Walker, value *ast.Value)) {
	o.value = append(o.value, f)
}

func Walk(schema *ast.Schema, document *ast.QueryDocument, observers *Events) {
	w := Walker{
		Observers: observers,
		Schema:    schema,
		Document:  document,
	}

	w.walk()
}

type Walker struct {
	Context   context.Context
	Observers *Events
	Schema    *ast.Schema
	Document  *ast.QueryDocument

	validatedFragmentSpreads map[string]bool
	CurrentOperation         *ast.OperationDefinition
}

func (w *Walker) walk() {
	for _, child := range w.Document.Operations {
		w.validatedFragmentSpreads = make(map[string]bool)
		w.walkOperation(child)
	}
	for _, child := range w.Document.Fragments {
		w.validatedFragmentSpreads = make(map[string]bool)
		w.walkFragment(child)
	}
}

func (w *Walker) walkOperation(operation *ast.OperationDefinition) {
	w.CurrentOperation = operation
	for _, varDef := range operation.VariableDefinitions {
		varDef.Definition = w.Schema.Types[varDef.Type.Name()]

		if varDef.DefaultValue != nil {
			varDef.DefaultValue.ExpectedType = varDef.Type
			varDef.DefaultValue.Definition = w.Schema.Types[varDef.Type.Name()]
		}
	}

	var def *ast.Definition
	var loc ast.DirectiveLocation
	switch operation.Operation {
	case ast.Query, "":
		def = w.Schema.Query
		loc = ast.LocationQuery
	case ast.Mutation:
		def = w.Schema.Mutation
		loc = ast.LocationMutation
	case ast.Subscription:
		def = w.Schema.Subscription
		loc = ast.LocationSubscription
	}

	w.walkDirectives(def, operation.Directives, loc)

	for _, varDef := range operation.VariableDefinitions {
		if varDef.DefaultValue != nil {
			w.walkValue(varDef.DefaultValue)
		}
	}

	w.walkSelectionSet(def, operation.SelectionSet)

	for _, v := range w.Observers.operationVisitor {
		v(w, operation)
	}
	w.CurrentOperation = nil
}

func (w *Walker) walkFragment(it *ast.FragmentDefinition) {
	def := w.Schema.Types[it.TypeCondition]

	it.Definition = def

	w.walkDirectives(def, it.Directives, ast.LocationFragmentDefinition)
	w.walkSelectionSet(def, it.SelectionSet)

	for _, v := range w.Observers.fragment {
		v(w, it)
	}
}

func (w *Walker) walkDirectives(parentDef *ast.Definition, directives []*ast.Directive, location ast.DirectiveLocation) {
	for _, dir := range directives {
		def := w.Schema.Directives[dir.Name]
		dir.Definition = def
		dir.ParentDefinition = parentDef
		dir.Location = location

		for _, arg := range dir.Arguments {
			var argDef *ast.ArgumentDefinition
			if def != nil {
				argDef = def.Arguments.ForName(arg.Name)
			}

			w.walkArgument(argDef, arg)
		}

		for _, v := range w.Observers.directive {
			v(w, dir)
		}
	}

	for _, v := range w.Observers.directiveList {
		v(w, directives)
	}
}

func (w *Walker) walkValue(value *ast.Value) {
	if value.Kind == ast.Variable && w.CurrentOperation != nil {
		value.VariableDefinition = w.CurrentOperation.VariableDefinitions.ForName(value.Raw)
		if value.VariableDefinition != nil {
			value.VariableDefinition.Used = true
		}
	}

	if value.Kind == ast.ObjectValue {
		for _, child := range value.Children {
			if value.Definition != nil {
				fieldDef := value.Definition.Fields.ForName(child.Name)
				if fieldDef != nil {
					child.Value.ExpectedType = fieldDef.Type
					child.Value.Definition = w.Schema.Types[fieldDef.Type.Name()]
				}
			}
			w.walkValue(child.Value)
		}
	}

	if value.Kind == ast.ListValue {
		for _, child := range value.Children {
			if value.ExpectedType != nil && value.ExpectedType.Elem != nil {
				child.Value.ExpectedType = value.ExpectedType.Elem
				child.Value.Definition = value.Definition
			}

			w.walkValue(child.Value)
		}
	}

	for _, v := range w.Observers.value {
		v(w, value)
	}
}

func (w *Walker) walkArgument(argDef *ast.ArgumentDefinition, arg *ast.Argument) {
	if argDef != nil {
		arg.Value.ExpectedType = argDef.Type
		arg.Value.Definition = w.Schema.Types[argDef.Type.Name()]
	}

	w.walkValue(arg.Value)
}

func (w *Walker) walkSelectionSet(parentDef *ast.Definition, it ast.SelectionSet) {
	for _, child := range it {
		w.walkSelection(parentDef, child)
	}
}

func (w *Walker) walkSelection(parentDef *ast.Definition, it ast.Selection) {
	switch it := it.(type) {
	case *ast.Field:
		var def *ast.FieldDefinition
		if it.Name == "__typename" {
			def = &ast.FieldDefinition{
				Name: "__typename",
				Type: ast.NamedType("String", nil),
			}
		} else if parentDef != nil {
			def = parentDef.Fields.ForName(it.Name)
		}

		it.Definition = def
		it.ObjectDefinition = parentDef

		var nextParentDef *ast.Definition
		if def != nil {
			nextParentDef = w.Schema.Types[def.Type.Name()]
		}

		for _, arg := range it.Arguments {
			var argDef *ast.ArgumentDefinition
			if def != nil {
				argDef = def.Arguments.ForName(arg.Name)
			}

			w.walkArgument(argDef, arg)
		}

		w.walkDirectives(nextParentDef, it.Directives, ast.LocationField)
		w.walkSelectionSet(nextParentDef, it.SelectionSet)

		for _, v := range w.Observers.field {
			v(w, it)
		}

	case *ast.InlineFragment:
		it.ObjectDefinition = parentDef

		nextParentDef := parentDef
		if it.TypeCondition != "" {
			nextParentDef = w.Schema.Types[it.TypeCondition]
		}

		w.walkDirectives(nextParentDef, it.Directives, ast.LocationInlineFragment)
		w.walkSelectionSet(nextParentDef, it.SelectionSet)

		for _, v := range w.Observers.inlineFragment {
			v(w, it)
		}

	case *ast.FragmentSpread:
		def := w.Document.Fragments.ForName(it.Name)
		it.Definition = def
		it.ObjectDefinition = parentDef

		var nextParentDef *ast.Definition
		if def != nil {
			nextParentDef = w.Schema.Types[def.TypeCondition]
		}

		w.walkDirectives(nextParentDef, it.Directives, ast.LocationFragmentSpread)

		if def != nil && !w.validatedFragmentSpreads[def.Name] {
			// prevent inifinite recursion
			w.validatedFragmentSpreads[def.Name] = true
			w.walkSelectionSet(nextParentDef, def.SelectionSet)
		}

		for _, v := range w.Observers.fragmentSpread {
			v(w, it)
		}

	default:
		panic(fmt.Errorf("unsupported %T", it))
	}
}
