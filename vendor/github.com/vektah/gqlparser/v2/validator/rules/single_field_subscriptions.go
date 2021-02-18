package validator

import (
	"strconv"

	"github.com/vektah/gqlparser/v2/ast"
	. "github.com/vektah/gqlparser/v2/validator"
)

func init() {
	AddRule("SingleFieldSubscriptions", func(observers *Events, addError AddErrFunc) {
		observers.OnOperation(func(walker *Walker, operation *ast.OperationDefinition) {
			if operation.Operation != ast.Subscription {
				return
			}

			if len(operation.SelectionSet) > 1 {
				name := "Anonymous Subscription"
				if operation.Name != "" {
					name = `Subscription ` + strconv.Quote(operation.Name)
				}

				addError(
					Message(`%s must select only one top level field.`, name),
					At(operation.SelectionSet[1].GetPosition()),
				)
			}
		})
	})
}
