//go:generate go run github.com/matryer/moq -out executable_schema_mock.go . ExecutableSchema

package graphql

import (
	"context"
	"fmt"

	"github.com/vektah/gqlparser/v2/ast"
)

type ExecutableSchema interface {
	Schema() *ast.Schema

	Complexity(typeName, fieldName string, childComplexity int, args map[string]interface{}) (int, bool)
	Exec(ctx context.Context) ResponseHandler
}

// CollectFields returns the set of fields from an ast.SelectionSet where all collected fields satisfy at least one of the GraphQL types
// passed through satisfies. Providing an empty or nil slice for satisfies will return collect all fields regardless of fragment
// type conditions.
func CollectFields(reqCtx *OperationContext, selSet ast.SelectionSet, satisfies []string) []CollectedField {
	return collectFields(reqCtx, selSet, satisfies, map[string]bool{})
}

func collectFields(reqCtx *OperationContext, selSet ast.SelectionSet, satisfies []string, visited map[string]bool) []CollectedField {
	groupedFields := make([]CollectedField, 0, len(selSet))

	for _, sel := range selSet {
		switch sel := sel.(type) {
		case *ast.Field:
			if !shouldIncludeNode(sel.Directives, reqCtx.Variables) {
				continue
			}
			f := getOrCreateAndAppendField(&groupedFields, sel.Alias, sel.ObjectDefinition, func() CollectedField {
				return CollectedField{Field: sel}
			})

			f.Selections = append(f.Selections, sel.SelectionSet...)
		case *ast.InlineFragment:
			if !shouldIncludeNode(sel.Directives, reqCtx.Variables) {
				continue
			}
			if len(satisfies) > 0 && !instanceOf(sel.TypeCondition, satisfies) {
				continue
			}
			for _, childField := range collectFields(reqCtx, sel.SelectionSet, satisfies, visited) {
				f := getOrCreateAndAppendField(&groupedFields, childField.Name, childField.ObjectDefinition, func() CollectedField { return childField })
				f.Selections = append(f.Selections, childField.Selections...)
			}

		case *ast.FragmentSpread:
			if !shouldIncludeNode(sel.Directives, reqCtx.Variables) {
				continue
			}
			fragmentName := sel.Name
			if _, seen := visited[fragmentName]; seen {
				continue
			}
			visited[fragmentName] = true

			fragment := reqCtx.Doc.Fragments.ForName(fragmentName)
			if fragment == nil {
				// should never happen, validator has already run
				panic(fmt.Errorf("missing fragment %s", fragmentName))
			}

			if len(satisfies) > 0 && !instanceOf(fragment.TypeCondition, satisfies) {
				continue
			}

			for _, childField := range collectFields(reqCtx, fragment.SelectionSet, satisfies, visited) {
				f := getOrCreateAndAppendField(&groupedFields, childField.Name, childField.ObjectDefinition, func() CollectedField { return childField })
				f.Selections = append(f.Selections, childField.Selections...)
			}
		default:
			panic(fmt.Errorf("unsupported %T", sel))
		}
	}

	return groupedFields
}

type CollectedField struct {
	*ast.Field

	Selections ast.SelectionSet
}

func instanceOf(val string, satisfies []string) bool {
	for _, s := range satisfies {
		if val == s {
			return true
		}
	}
	return false
}

func getOrCreateAndAppendField(c *[]CollectedField, name string, objectDefinition *ast.Definition, creator func() CollectedField) *CollectedField {
	for i, cf := range *c {
		if cf.Alias == name && (cf.ObjectDefinition == objectDefinition || (cf.ObjectDefinition != nil && objectDefinition != nil && cf.ObjectDefinition.Name == objectDefinition.Name)) {
			return &(*c)[i]
		}
	}

	f := creator()

	*c = append(*c, f)
	return &(*c)[len(*c)-1]
}

func shouldIncludeNode(directives ast.DirectiveList, variables map[string]interface{}) bool {
	if len(directives) == 0 {
		return true
	}

	skip, include := false, true

	if d := directives.ForName("skip"); d != nil {
		skip = resolveIfArgument(d, variables)
	}

	if d := directives.ForName("include"); d != nil {
		include = resolveIfArgument(d, variables)
	}

	return !skip && include
}

func resolveIfArgument(d *ast.Directive, variables map[string]interface{}) bool {
	arg := d.Arguments.ForName("if")
	if arg == nil {
		panic(fmt.Sprintf("%s: argument 'if' not defined", d.Name))
	}
	value, err := arg.Value.Value(variables)
	if err != nil {
		panic(err)
	}
	ret, ok := value.(bool)
	if !ok {
		panic(fmt.Sprintf("%s: argument 'if' is not a boolean", d.Name))
	}
	return ret
}
