package validator

import (
	"fmt"
	"sort"

	"github.com/vektah/gqlparser/v2/ast"
	. "github.com/vektah/gqlparser/v2/validator"
)

func init() {
	AddRule("FieldsOnCorrectType", func(observers *Events, addError AddErrFunc) {
		observers.OnField(func(walker *Walker, field *ast.Field) {
			if field.ObjectDefinition == nil || field.Definition != nil {
				return
			}

			message := fmt.Sprintf(`Cannot query field "%s" on type "%s".`, field.Name, field.ObjectDefinition.Name)

			if suggestedTypeNames := getSuggestedTypeNames(walker, field.ObjectDefinition, field.Name); suggestedTypeNames != nil {
				message += " Did you mean to use an inline fragment on " + QuotedOrList(suggestedTypeNames...) + "?"
			} else if suggestedFieldNames := getSuggestedFieldNames(field.ObjectDefinition, field.Name); suggestedFieldNames != nil {
				message += " Did you mean " + QuotedOrList(suggestedFieldNames...) + "?"
			}

			addError(
				Message(message),
				At(field.Position),
			)
		})
	})
}

// Go through all of the implementations of type, as well as the interfaces
// that they implement. If any of those types include the provided field,
// suggest them, sorted by how often the type is referenced,  starting
// with Interfaces.
func getSuggestedTypeNames(walker *Walker, parent *ast.Definition, name string) []string {
	if !parent.IsAbstractType() {
		return nil
	}

	var suggestedObjectTypes []string
	var suggestedInterfaceTypes []string
	interfaceUsageCount := map[string]int{}

	for _, possibleType := range walker.Schema.GetPossibleTypes(parent) {
		field := possibleType.Fields.ForName(name)
		if field == nil {
			continue
		}

		suggestedObjectTypes = append(suggestedObjectTypes, possibleType.Name)

		for _, possibleInterface := range possibleType.Interfaces {
			interfaceField := walker.Schema.Types[possibleInterface]
			if interfaceField != nil && interfaceField.Fields.ForName(name) != nil {
				if interfaceUsageCount[possibleInterface] == 0 {
					suggestedInterfaceTypes = append(suggestedInterfaceTypes, possibleInterface)
				}
				interfaceUsageCount[possibleInterface]++
			}
		}
	}

	sort.SliceStable(suggestedInterfaceTypes, func(i, j int) bool {
		return interfaceUsageCount[suggestedInterfaceTypes[i]] > interfaceUsageCount[suggestedInterfaceTypes[j]]
	})

	return append(suggestedInterfaceTypes, suggestedObjectTypes...)
}

// For the field name provided, determine if there are any similar field names
// that may be the result of a typo.
func getSuggestedFieldNames(parent *ast.Definition, name string) []string {
	if parent.Kind != ast.Object && parent.Kind != ast.Interface {
		return nil
	}

	var possibleFieldNames []string
	for _, field := range parent.Fields {
		possibleFieldNames = append(possibleFieldNames, field.Name)
	}

	return SuggestionList(name, possibleFieldNames)
}
