package validator

import (
	"github.com/vektah/gqlparser/v2/ast"
	. "github.com/vektah/gqlparser/v2/validator"
)

func init() {
	AddRule("ProvidedRequiredArguments", func(observers *Events, addError AddErrFunc) {

		observers.OnField(func(walker *Walker, field *ast.Field) {
			if field.Definition == nil {
				return
			}

		argDef:
			for _, argDef := range field.Definition.Arguments {
				if !argDef.Type.NonNull {
					continue
				}
				if argDef.DefaultValue != nil {
					continue
				}
				for _, arg := range field.Arguments {
					if arg.Name == argDef.Name {
						continue argDef
					}
				}

				addError(
					Message(`Field "%s" argument "%s" of type "%s" is required but not provided.`, field.Name, argDef.Name, argDef.Type.String()),
					At(field.Position),
				)
			}
		})

		observers.OnDirective(func(walker *Walker, directive *ast.Directive) {
			if directive.Definition == nil {
				return
			}

		argDef:
			for _, argDef := range directive.Definition.Arguments {
				if !argDef.Type.NonNull {
					continue
				}
				if argDef.DefaultValue != nil {
					continue
				}
				for _, arg := range directive.Arguments {
					if arg.Name == argDef.Name {
						continue argDef
					}
				}

				addError(
					Message(`Directive "@%s" argument "%s" of type "%s" is required but not provided.`, directive.Definition.Name, argDef.Name, argDef.Type.String()),
					At(directive.Position),
				)
			}
		})
	})
}
