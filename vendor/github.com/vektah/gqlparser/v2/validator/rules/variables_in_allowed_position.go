package validator

import (
	"github.com/vektah/gqlparser/v2/ast"
	. "github.com/vektah/gqlparser/v2/validator"
)

func init() {
	AddRule("VariablesInAllowedPosition", func(observers *Events, addError AddErrFunc) {
		observers.OnValue(func(walker *Walker, value *ast.Value) {
			if value.Kind != ast.Variable || value.ExpectedType == nil || value.VariableDefinition == nil || walker.CurrentOperation == nil {
				return
			}

			// todo: move me into walk
			// If there is a default non nullable types can be null
			if value.VariableDefinition.DefaultValue != nil && value.VariableDefinition.DefaultValue.Kind != ast.NullValue {
				if value.ExpectedType.NonNull {
					value.ExpectedType.NonNull = false
				}
			}

			if !value.VariableDefinition.Type.IsCompatible(value.ExpectedType) {
				addError(
					Message(
						`Variable "%s" of type "%s" used in position expecting type "%s".`,
						value,
						value.VariableDefinition.Type.String(),
						value.ExpectedType.String(),
					),
					At(value.Position),
				)
			}
		})
	})
}
