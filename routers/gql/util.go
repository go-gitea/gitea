package gql

import (
	"fmt"

	"github.com/graphql-go/graphql"
	ast "github.com/graphql-go/graphql/language/ast"
)

func getSelectedFields(params graphql.ResolveParams) ([]string, error) {
	fieldASTs := params.Info.FieldASTs
	if len(fieldASTs) == 0 {
		return nil, fmt.Errorf("getSelectedFields: ResolveParams has no fields")
	}
	return selectedFieldsFromSelections(params, fieldASTs[0].SelectionSet.Selections)
}

func selectedFieldsFromSelections(params graphql.ResolveParams, selections []ast.Selection) ([]string, error) {
	var selected []string
	for _, s := range selections {
		switch t := s.(type) {
		case *ast.Field:
			selected = append(selected, s.(*ast.Field).Name.Value)
		case *ast.FragmentSpread:
			n := s.(*ast.FragmentSpread).Name.Value
			frag, ok := params.Info.Fragments[n]
			if !ok {
				return nil, fmt.Errorf("getSelectedFields: no fragment found with name %v", n)
			}
			sel, err := selectedFieldsFromSelections(params, frag.GetSelectionSet().Selections)
			if err != nil {
				return nil, err
			}
			selected = append(selected, sel...)
		default:
			return nil, fmt.Errorf("getSelectedFields: found unexpected selection type %v", t)
		}
	}
	return selected, nil
}
