package validator

import (
	"github.com/vektah/gqlparser/v2/ast"
	. "github.com/vektah/gqlparser/v2/validator"
)

func init() {
	AddRule("UniqueInputFieldNames", func(observers *Events, addError AddErrFunc) {
		observers.OnValue(func(walker *Walker, value *ast.Value) {
			if value.Kind != ast.ObjectValue {
				return
			}

			seen := map[string]bool{}
			for _, field := range value.Children {
				if seen[field.Name] {
					addError(
						Message(`There can be only one input field named "%s".`, field.Name),
						At(field.Position),
					)
				}
				seen[field.Name] = true
			}
		})
	})
}
