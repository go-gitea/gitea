package validator

import (
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	. "github.com/vektah/gqlparser/v2/validator"
)

func init() {
	AddRule("NoFragmentCycles", func(observers *Events, addError AddErrFunc) {
		visitedFrags := make(map[string]bool)

		observers.OnFragment(func(walker *Walker, fragment *ast.FragmentDefinition) {
			var spreadPath []*ast.FragmentSpread
			spreadPathIndexByName := make(map[string]int)

			var recursive func(fragment *ast.FragmentDefinition)
			recursive = func(fragment *ast.FragmentDefinition) {
				if visitedFrags[fragment.Name] {
					return
				}

				visitedFrags[fragment.Name] = true

				spreadNodes := getFragmentSpreads(fragment.SelectionSet)
				if len(spreadNodes) == 0 {
					return
				}
				spreadPathIndexByName[fragment.Name] = len(spreadPath)

				for _, spreadNode := range spreadNodes {
					spreadName := spreadNode.Name

					cycleIndex, ok := spreadPathIndexByName[spreadName]

					spreadPath = append(spreadPath, spreadNode)
					if !ok {
						spreadFragment := walker.Document.Fragments.ForName(spreadName)
						if spreadFragment != nil {
							recursive(spreadFragment)
						}
					} else {
						cyclePath := spreadPath[cycleIndex : len(spreadPath)-1]
						var fragmentNames []string
						for _, fs := range cyclePath {
							fragmentNames = append(fragmentNames, fs.Name)
						}
						var via string
						if len(fragmentNames) != 0 {
							via = fmt.Sprintf(" via %s", strings.Join(fragmentNames, ", "))
						}
						addError(
							Message(`Cannot spread fragment "%s" within itself%s.`, spreadName, via),
							At(spreadNode.Position),
						)
					}

					spreadPath = spreadPath[:len(spreadPath)-1]
				}

				delete(spreadPathIndexByName, fragment.Name)
			}

			recursive(fragment)
		})
	})
}

func getFragmentSpreads(node ast.SelectionSet) []*ast.FragmentSpread {
	var spreads []*ast.FragmentSpread

	setsToVisit := []ast.SelectionSet{node}

	for len(setsToVisit) != 0 {
		set := setsToVisit[len(setsToVisit)-1]
		setsToVisit = setsToVisit[:len(setsToVisit)-1]

		for _, selection := range set {
			switch selection := selection.(type) {
			case *ast.FragmentSpread:
				spreads = append(spreads, selection)
			case *ast.Field:
				setsToVisit = append(setsToVisit, selection.SelectionSet)
			case *ast.InlineFragment:
				setsToVisit = append(setsToVisit, selection.SelectionSet)
			}
		}
	}

	return spreads
}
