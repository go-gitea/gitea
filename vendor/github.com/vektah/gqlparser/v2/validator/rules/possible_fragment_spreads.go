package validator

import (
	"github.com/vektah/gqlparser/v2/ast"
	. "github.com/vektah/gqlparser/v2/validator"
)

func init() {
	AddRule("PossibleFragmentSpreads", func(observers *Events, addError AddErrFunc) {

		validate := func(walker *Walker, parentDef *ast.Definition, fragmentName string, emitError func()) {
			if parentDef == nil {
				return
			}

			var parentDefs []*ast.Definition
			switch parentDef.Kind {
			case ast.Object:
				parentDefs = []*ast.Definition{parentDef}
			case ast.Interface, ast.Union:
				parentDefs = walker.Schema.GetPossibleTypes(parentDef)
			default:
				return
			}

			fragmentDefType := walker.Schema.Types[fragmentName]
			if fragmentDefType == nil {
				return
			}
			if !fragmentDefType.IsCompositeType() {
				// checked by FragmentsOnCompositeTypes
				return
			}
			fragmentDefs := walker.Schema.GetPossibleTypes(fragmentDefType)

			for _, fragmentDef := range fragmentDefs {
				for _, parentDef := range parentDefs {
					if parentDef.Name == fragmentDef.Name {
						return
					}
				}
			}

			emitError()
		}

		observers.OnInlineFragment(func(walker *Walker, inlineFragment *ast.InlineFragment) {
			validate(walker, inlineFragment.ObjectDefinition, inlineFragment.TypeCondition, func() {
				addError(
					Message(`Fragment cannot be spread here as objects of type "%s" can never be of type "%s".`, inlineFragment.ObjectDefinition.Name, inlineFragment.TypeCondition),
					At(inlineFragment.Position),
				)
			})
		})

		observers.OnFragmentSpread(func(walker *Walker, fragmentSpread *ast.FragmentSpread) {
			if fragmentSpread.Definition == nil {
				return
			}
			validate(walker, fragmentSpread.ObjectDefinition, fragmentSpread.Definition.TypeCondition, func() {
				addError(
					Message(`Fragment "%s" cannot be spread here as objects of type "%s" can never be of type "%s".`, fragmentSpread.Name, fragmentSpread.ObjectDefinition.Name, fragmentSpread.Definition.TypeCondition),
					At(fragmentSpread.Position),
				)
			})
		})
	})
}
