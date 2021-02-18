package validator

import (
	"github.com/vektah/gqlparser/v2/ast"
	. "github.com/vektah/gqlparser/v2/validator"
)

func init() {
	AddRule("ScalarLeafs", func(observers *Events, addError AddErrFunc) {
		observers.OnField(func(walker *Walker, field *ast.Field) {
			if field.Definition == nil {
				return
			}

			fieldType := walker.Schema.Types[field.Definition.Type.Name()]
			if fieldType == nil {
				return
			}

			if fieldType.IsLeafType() && len(field.SelectionSet) > 0 {
				addError(
					Message(`Field "%s" must not have a selection since type "%s" has no subfields.`, field.Name, fieldType.Name),
					At(field.Position),
				)
			}

			if !fieldType.IsLeafType() && len(field.SelectionSet) == 0 {
				addError(
					Message(`Field "%s" of type "%s" must have a selection of subfields.`, field.Name, field.Definition.Type.String()),
					Suggestf(`"%s { ... }"`, field.Name),
					At(field.Position),
				)
			}
		})
	})
}
