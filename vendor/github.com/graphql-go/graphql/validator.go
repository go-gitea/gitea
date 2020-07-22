package graphql

import (
	"github.com/graphql-go/graphql/gqlerrors"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/kinds"
	"github.com/graphql-go/graphql/language/visitor"
)

type ValidationResult struct {
	IsValid bool
	Errors  []gqlerrors.FormattedError
}

/**
 * Implements the "Validation" section of the spec.
 *
 * Validation runs synchronously, returning an array of encountered errors, or
 * an empty array if no errors were encountered and the document is valid.
 *
 * A list of specific validation rules may be provided. If not provided, the
 * default list of rules defined by the GraphQL specification will be used.
 *
 * Each validation rules is a function which returns a visitor
 * (see the language/visitor API). Visitor methods are expected to return
 * GraphQLErrors, or Arrays of GraphQLErrors when invalid.
 */

func ValidateDocument(schema *Schema, astDoc *ast.Document, rules []ValidationRuleFn) (vr ValidationResult) {
	if len(rules) == 0 {
		rules = SpecifiedRules
	}

	if schema == nil {
		vr.Errors = append(vr.Errors, gqlerrors.NewFormattedError("Must provide schema"))
		return vr
	}
	if astDoc == nil {
		vr.Errors = append(vr.Errors, gqlerrors.NewFormattedError("Must provide document"))
		return vr
	}

	typeInfo := NewTypeInfo(&TypeInfoConfig{
		Schema: schema,
	})
	vr.Errors = VisitUsingRules(schema, typeInfo, astDoc, rules)
	if len(vr.Errors) == 0 {
		vr.IsValid = true
	}
	return vr
}

// VisitUsingRules This uses a specialized visitor which runs multiple visitors in parallel,
// while maintaining the visitor skip and break API.
//
// @internal
// Had to expose it to unit test experimental customizable validation feature,
// but not meant for public consumption
func VisitUsingRules(schema *Schema, typeInfo *TypeInfo, astDoc *ast.Document, rules []ValidationRuleFn) []gqlerrors.FormattedError {

	context := NewValidationContext(schema, astDoc, typeInfo)
	visitors := []*visitor.VisitorOptions{}

	for _, rule := range rules {
		instance := rule(context)
		visitors = append(visitors, instance.VisitorOpts)
	}

	// Visit the whole document with each instance of all provided rules.
	visitor.Visit(astDoc, visitor.VisitWithTypeInfo(typeInfo, visitor.VisitInParallel(visitors...)), nil)
	return context.Errors()
}

type HasSelectionSet interface {
	GetKind() string
	GetLoc() *ast.Location
	GetSelectionSet() *ast.SelectionSet
}

var _ HasSelectionSet = (*ast.OperationDefinition)(nil)
var _ HasSelectionSet = (*ast.FragmentDefinition)(nil)

type VariableUsage struct {
	Node *ast.Variable
	Type Input
}

type ValidationContext struct {
	schema                         *Schema
	astDoc                         *ast.Document
	typeInfo                       *TypeInfo
	errors                         []gqlerrors.FormattedError
	fragments                      map[string]*ast.FragmentDefinition
	variableUsages                 map[HasSelectionSet][]*VariableUsage
	recursiveVariableUsages        map[*ast.OperationDefinition][]*VariableUsage
	recursivelyReferencedFragments map[*ast.OperationDefinition][]*ast.FragmentDefinition
	fragmentSpreads                map[*ast.SelectionSet][]*ast.FragmentSpread
}

func NewValidationContext(schema *Schema, astDoc *ast.Document, typeInfo *TypeInfo) *ValidationContext {
	return &ValidationContext{
		schema:                         schema,
		astDoc:                         astDoc,
		typeInfo:                       typeInfo,
		fragments:                      map[string]*ast.FragmentDefinition{},
		variableUsages:                 map[HasSelectionSet][]*VariableUsage{},
		recursiveVariableUsages:        map[*ast.OperationDefinition][]*VariableUsage{},
		recursivelyReferencedFragments: map[*ast.OperationDefinition][]*ast.FragmentDefinition{},
		fragmentSpreads:                map[*ast.SelectionSet][]*ast.FragmentSpread{},
	}
}

func (ctx *ValidationContext) ReportError(err error) {
	formattedErr := gqlerrors.FormatError(err)
	ctx.errors = append(ctx.errors, formattedErr)
}
func (ctx *ValidationContext) Errors() []gqlerrors.FormattedError {
	return ctx.errors
}

func (ctx *ValidationContext) Schema() *Schema {
	return ctx.schema
}
func (ctx *ValidationContext) Document() *ast.Document {
	return ctx.astDoc
}
func (ctx *ValidationContext) Fragment(name string) *ast.FragmentDefinition {
	if len(ctx.fragments) == 0 {
		if ctx.Document() == nil {
			return nil
		}
		defs := ctx.Document().Definitions
		fragments := map[string]*ast.FragmentDefinition{}
		for _, def := range defs {
			if def, ok := def.(*ast.FragmentDefinition); ok {
				defName := ""
				if def.Name != nil {
					defName = def.Name.Value
				}
				fragments[defName] = def
			}
		}
		ctx.fragments = fragments
	}
	f, _ := ctx.fragments[name]
	return f
}
func (ctx *ValidationContext) FragmentSpreads(node *ast.SelectionSet) []*ast.FragmentSpread {
	if spreads, ok := ctx.fragmentSpreads[node]; ok && spreads != nil {
		return spreads
	}

	spreads := []*ast.FragmentSpread{}
	setsToVisit := []*ast.SelectionSet{node}

	for {
		if len(setsToVisit) == 0 {
			break
		}
		var set *ast.SelectionSet
		// pop
		set, setsToVisit = setsToVisit[len(setsToVisit)-1], setsToVisit[:len(setsToVisit)-1]
		if set.Selections != nil {
			for _, selection := range set.Selections {
				switch selection := selection.(type) {
				case *ast.FragmentSpread:
					spreads = append(spreads, selection)
				case *ast.Field:
					if selection.SelectionSet != nil {
						setsToVisit = append(setsToVisit, selection.SelectionSet)
					}
				case *ast.InlineFragment:
					if selection.SelectionSet != nil {
						setsToVisit = append(setsToVisit, selection.SelectionSet)
					}
				}
			}
		}
		ctx.fragmentSpreads[node] = spreads
	}
	return spreads
}

func (ctx *ValidationContext) RecursivelyReferencedFragments(operation *ast.OperationDefinition) []*ast.FragmentDefinition {
	if fragments, ok := ctx.recursivelyReferencedFragments[operation]; ok && fragments != nil {
		return fragments
	}

	fragments := []*ast.FragmentDefinition{}
	collectedNames := map[string]bool{}
	nodesToVisit := []*ast.SelectionSet{operation.SelectionSet}

	for {
		if len(nodesToVisit) == 0 {
			break
		}

		var node *ast.SelectionSet

		node, nodesToVisit = nodesToVisit[len(nodesToVisit)-1], nodesToVisit[:len(nodesToVisit)-1]
		spreads := ctx.FragmentSpreads(node)
		for _, spread := range spreads {
			fragName := ""
			if spread.Name != nil {
				fragName = spread.Name.Value
			}
			if res, ok := collectedNames[fragName]; !ok || !res {
				collectedNames[fragName] = true
				fragment := ctx.Fragment(fragName)
				if fragment != nil {
					fragments = append(fragments, fragment)
					nodesToVisit = append(nodesToVisit, fragment.SelectionSet)
				}
			}

		}
	}

	ctx.recursivelyReferencedFragments[operation] = fragments
	return fragments
}
func (ctx *ValidationContext) VariableUsages(node HasSelectionSet) []*VariableUsage {
	if usages, ok := ctx.variableUsages[node]; ok && usages != nil {
		return usages
	}
	usages := []*VariableUsage{}
	typeInfo := NewTypeInfo(&TypeInfoConfig{
		Schema: ctx.schema,
	})

	visitor.Visit(node, visitor.VisitWithTypeInfo(typeInfo, &visitor.VisitorOptions{
		KindFuncMap: map[string]visitor.NamedVisitFuncs{
			kinds.VariableDefinition: {
				Kind: func(p visitor.VisitFuncParams) (string, interface{}) {
					return visitor.ActionSkip, nil
				},
			},
			kinds.Variable: {
				Kind: func(p visitor.VisitFuncParams) (string, interface{}) {
					if node, ok := p.Node.(*ast.Variable); ok && node != nil {
						usages = append(usages, &VariableUsage{
							Node: node,
							Type: typeInfo.InputType(),
						})
					}
					return visitor.ActionNoChange, nil
				},
			},
		},
	}), nil)

	ctx.variableUsages[node] = usages
	return usages
}
func (ctx *ValidationContext) RecursiveVariableUsages(operation *ast.OperationDefinition) []*VariableUsage {
	if usages, ok := ctx.recursiveVariableUsages[operation]; ok && usages != nil {
		return usages
	}
	usages := ctx.VariableUsages(operation)

	fragments := ctx.RecursivelyReferencedFragments(operation)
	for _, fragment := range fragments {
		fragmentUsages := ctx.VariableUsages(fragment)
		usages = append(usages, fragmentUsages...)
	}

	ctx.recursiveVariableUsages[operation] = usages
	return usages
}
func (ctx *ValidationContext) Type() Output {
	return ctx.typeInfo.Type()
}
func (ctx *ValidationContext) ParentType() Composite {
	return ctx.typeInfo.ParentType()
}
func (ctx *ValidationContext) InputType() Input {
	return ctx.typeInfo.InputType()
}
func (ctx *ValidationContext) FieldDef() *FieldDefinition {
	return ctx.typeInfo.FieldDef()
}
func (ctx *ValidationContext) Directive() *Directive {
	return ctx.typeInfo.Directive()
}
func (ctx *ValidationContext) Argument() *Argument {
	return ctx.typeInfo.Argument()
}
