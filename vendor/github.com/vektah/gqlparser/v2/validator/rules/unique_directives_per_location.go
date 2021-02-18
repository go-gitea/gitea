package validator

import (
	"github.com/vektah/gqlparser/v2/ast"
	. "github.com/vektah/gqlparser/v2/validator"
)

func init() {
	AddRule("UniqueDirectivesPerLocation", func(observers *Events, addError AddErrFunc) {
		observers.OnDirectiveList(func(walker *Walker, directives []*ast.Directive) {
			seen := map[string]bool{}

			for _, dir := range directives {
				if seen[dir.Name] {
					addError(
						Message(`The directive "%s" can only be used once at this location.`, dir.Name),
						At(dir.Position),
					)
				}
				seen[dir.Name] = true
			}
		})
	})
}
