package graphql

import (
	"fmt"
	"strings"

	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/kinds"
	"github.com/graphql-go/graphql/language/printer"
	"github.com/graphql-go/graphql/language/visitor"
)

func fieldsConflictMessage(responseName string, reason conflictReason) string {
	return fmt.Sprintf(`Fields "%v" conflict because %v. `+
		`Use different aliases on the fields to fetch both if this was intentional.`,
		responseName,
		fieldsConflictReasonMessage(reason),
	)
}

func fieldsConflictReasonMessage(message interface{}) string {
	switch reason := message.(type) {
	case string:
		return reason
	case conflictReason:
		return fieldsConflictReasonMessage(reason.Message)
	case []conflictReason:
		messages := []string{}
		for _, r := range reason {
			messages = append(messages, fmt.Sprintf(
				`subfields "%v" conflict because %v`,
				r.Name,
				fieldsConflictReasonMessage(r.Message),
			))
		}
		return strings.Join(messages, " and ")
	}
	return ""
}

// OverlappingFieldsCanBeMergedRule Overlapping fields can be merged
//
// A selection set is only valid if all fields (including spreading any
// fragments) either correspond to distinct response names or can be merged
// without ambiguity.
func OverlappingFieldsCanBeMergedRule(context *ValidationContext) *ValidationRuleInstance {

	// A memoization for when two fragments are compared "between" each other for
	// conflicts. Two fragments may be compared many times, so memoizing this can
	// dramatically improve the performance of this validator.
	comparedSet := newPairSet()

	// A cache for the "field map" and list of fragment names found in any given
	// selection set. Selection sets may be asked for this information multiple
	// times, so this improves the performance of this validator.
	cacheMap := map[*ast.SelectionSet]*fieldsAndFragmentNames{}

	visitorOpts := &visitor.VisitorOptions{
		KindFuncMap: map[string]visitor.NamedVisitFuncs{
			kinds.SelectionSet: {
				Kind: func(p visitor.VisitFuncParams) (string, interface{}) {
					if selectionSet, ok := p.Node.(*ast.SelectionSet); ok && selectionSet != nil {
						parentType, _ := context.ParentType().(Named)

						rule := &overlappingFieldsCanBeMergedRule{
							context:     context,
							comparedSet: comparedSet,
							cacheMap:    cacheMap,
						}
						conflicts := rule.findConflictsWithinSelectionSet(parentType, selectionSet)
						if len(conflicts) > 0 {
							for _, c := range conflicts {
								responseName := c.Reason.Name
								reason := c.Reason
								reportError(
									context,
									fieldsConflictMessage(responseName, reason),
									append(c.FieldsLeft, c.FieldsRight...),
								)
							}
							return visitor.ActionNoChange, nil
						}
					}
					return visitor.ActionNoChange, nil
				},
			},
		},
	}
	return &ValidationRuleInstance{
		VisitorOpts: visitorOpts,
	}
}

/**
 * Algorithm:
 *
 * Conflicts occur when two fields exist in a query which will produce the same
 * response name, but represent differing values, thus creating a conflict.
 * The algorithm below finds all conflicts via making a series of comparisons
 * between fields. In order to compare as few fields as possible, this makes
 * a series of comparisons "within" sets of fields and "between" sets of fields.
 *
 * Given any selection set, a collection produces both a set of fields by
 * also including all inline fragments, as well as a list of fragments
 * referenced by fragment spreads.
 *
 * A) Each selection set represented in the document first compares "within" its
 * collected set of fields, finding any conflicts between every pair of
 * overlapping fields.
 * Note: This is the *only time* that a the fields "within" a set are compared
 * to each other. After this only fields "between" sets are compared.
 *
 * B) Also, if any fragment is referenced in a selection set, then a
 * comparison is made "between" the original set of fields and the
 * referenced fragment.
 *
 * C) Also, if multiple fragments are referenced, then comparisons
 * are made "between" each referenced fragment.
 *
 * D) When comparing "between" a set of fields and a referenced fragment, first
 * a comparison is made between each field in the original set of fields and
 * each field in the the referenced set of fields.
 *
 * E) Also, if any fragment is referenced in the referenced selection set,
 * then a comparison is made "between" the original set of fields and the
 * referenced fragment (recursively referring to step D).
 *
 * F) When comparing "between" two fragments, first a comparison is made between
 * each field in the first referenced set of fields and each field in the the
 * second referenced set of fields.
 *
 * G) Also, any fragments referenced by the first must be compared to the
 * second, and any fragments referenced by the second must be compared to the
 * first (recursively referring to step F).
 *
 * H) When comparing two fields, if both have selection sets, then a comparison
 * is made "between" both selection sets, first comparing the set of fields in
 * the first selection set with the set of fields in the second.
 *
 * I) Also, if any fragment is referenced in either selection set, then a
 * comparison is made "between" the other set of fields and the
 * referenced fragment.
 *
 * J) Also, if two fragments are referenced in both selection sets, then a
 * comparison is made "between" the two fragments.
 *
 */

type overlappingFieldsCanBeMergedRule struct {
	context *ValidationContext

	// A memoization for when two fragments are compared "between" each other for
	// conflicts. Two fragments may be compared many times, so memoizing this can
	// dramatically improve the performance of this validator.
	comparedSet *pairSet

	// A cache for the "field map" and list of fragment names found in any given
	// selection set. Selection sets may be asked for this information multiple
	// times, so this improves the performance of this validator.
	cacheMap map[*ast.SelectionSet]*fieldsAndFragmentNames
}

// Find all conflicts found "within" a selection set, including those found
// via spreading in fragments. Called when visiting each SelectionSet in the
// GraphQL Document.
func (rule *overlappingFieldsCanBeMergedRule) findConflictsWithinSelectionSet(parentType Named, selectionSet *ast.SelectionSet) []conflict {
	conflicts := []conflict{}

	fieldsInfo := rule.getFieldsAndFragmentNames(parentType, selectionSet)

	// (A) Find find all conflicts "within" the fields of this selection set.
	// Note: this is the *only place* `collectConflictsWithin` is called.
	conflicts = rule.collectConflictsWithin(conflicts, fieldsInfo)

	// (B) Then collect conflicts between these fields and those represented by
	// each spread fragment name found.
	for i := 0; i < len(fieldsInfo.fragmentNames); i++ {

		conflicts = rule.collectConflictsBetweenFieldsAndFragment(conflicts, false, fieldsInfo, fieldsInfo.fragmentNames[i])

		// (C) Then compare this fragment with all other fragments found in this
		// selection set to collect conflicts between fragments spread together.
		// This compares each item in the list of fragment names to every other item
		// in that same list (except for itself).
		for k := i + 1; k < len(fieldsInfo.fragmentNames); k++ {
			conflicts = rule.collectConflictsBetweenFragments(conflicts, false, fieldsInfo.fragmentNames[i], fieldsInfo.fragmentNames[k])
		}
	}
	return conflicts
}

// Collect all conflicts found between a set of fields and a fragment reference
// including via spreading in any nested fragments.
func (rule *overlappingFieldsCanBeMergedRule) collectConflictsBetweenFieldsAndFragment(conflicts []conflict, areMutuallyExclusive bool, fieldsInfo *fieldsAndFragmentNames, fragmentName string) []conflict {
	fragment := rule.context.Fragment(fragmentName)
	if fragment == nil {
		return conflicts
	}

	fieldsInfo2 := rule.getReferencedFieldsAndFragmentNames(fragment)

	// (D) First collect any conflicts between the provided collection of fields
	// and the collection of fields represented by the given fragment.
	conflicts = rule.collectConflictsBetween(conflicts, areMutuallyExclusive, fieldsInfo, fieldsInfo2)

	// (E) Then collect any conflicts between the provided collection of fields
	// and any fragment names found in the given fragment.
	for _, fragmentName2 := range fieldsInfo2.fragmentNames {
		conflicts = rule.collectConflictsBetweenFieldsAndFragment(conflicts, areMutuallyExclusive, fieldsInfo2, fragmentName2)
	}

	return conflicts

}

// Collect all conflicts found between two fragments, including via spreading in
// any nested fragments.
func (rule *overlappingFieldsCanBeMergedRule) collectConflictsBetweenFragments(conflicts []conflict, areMutuallyExclusive bool, fragmentName1 string, fragmentName2 string) []conflict {
	fragment1 := rule.context.Fragment(fragmentName1)
	fragment2 := rule.context.Fragment(fragmentName2)

	if fragment1 == nil || fragment2 == nil {
		return conflicts
	}

	// No need to compare a fragment to itself.
	if fragment1 == fragment2 {
		return conflicts
	}

	// Memoize so two fragments are not compared for conflicts more than once.
	if rule.comparedSet.Has(fragmentName1, fragmentName2, areMutuallyExclusive) {
		return conflicts
	}
	rule.comparedSet.Add(fragmentName1, fragmentName2, areMutuallyExclusive)

	fieldsInfo1 := rule.getReferencedFieldsAndFragmentNames(fragment1)
	fieldsInfo2 := rule.getReferencedFieldsAndFragmentNames(fragment2)

	// (F) First, collect all conflicts between these two collections of fields
	// (not including any nested fragments).
	conflicts = rule.collectConflictsBetween(conflicts, areMutuallyExclusive, fieldsInfo1, fieldsInfo2)

	// (G) Then collect conflicts between the first fragment and any nested
	// fragments spread in the second fragment.
	for _, innerFragmentName2 := range fieldsInfo2.fragmentNames {
		conflicts = rule.collectConflictsBetweenFragments(conflicts, areMutuallyExclusive, fragmentName1, innerFragmentName2)
	}

	// (G) Then collect conflicts between the second fragment and any nested
	// fragments spread in the first fragment.
	for _, innerFragmentName1 := range fieldsInfo1.fragmentNames {
		conflicts = rule.collectConflictsBetweenFragments(conflicts, areMutuallyExclusive, innerFragmentName1, fragmentName2)
	}

	return conflicts
}

// Find all conflicts found between two selection sets, including those found
// via spreading in fragments. Called when determining if conflicts exist
// between the sub-fields of two overlapping fields.
func (rule *overlappingFieldsCanBeMergedRule) findConflictsBetweenSubSelectionSets(areMutuallyExclusive bool, parentType1 Named, selectionSet1 *ast.SelectionSet, parentType2 Named, selectionSet2 *ast.SelectionSet) []conflict {
	conflicts := []conflict{}

	fieldsInfo1 := rule.getFieldsAndFragmentNames(parentType1, selectionSet1)
	fieldsInfo2 := rule.getFieldsAndFragmentNames(parentType2, selectionSet2)

	// (H) First, collect all conflicts between these two collections of field.
	conflicts = rule.collectConflictsBetween(conflicts, areMutuallyExclusive, fieldsInfo1, fieldsInfo2)

	// (I) Then collect conflicts between the first collection of fields and
	// those referenced by each fragment name associated with the second.
	for _, fragmentName2 := range fieldsInfo2.fragmentNames {
		conflicts = rule.collectConflictsBetweenFieldsAndFragment(conflicts, areMutuallyExclusive, fieldsInfo1, fragmentName2)
	}

	// (I) Then collect conflicts between the second collection of fields and
	// those referenced by each fragment name associated with the first.
	for _, fragmentName1 := range fieldsInfo1.fragmentNames {
		conflicts = rule.collectConflictsBetweenFieldsAndFragment(conflicts, areMutuallyExclusive, fieldsInfo2, fragmentName1)
	}

	// (J) Also collect conflicts between any fragment names by the first and
	// fragment names by the second. This compares each item in the first set of
	// names to each item in the second set of names.
	for _, fragmentName1 := range fieldsInfo1.fragmentNames {
		for _, fragmentName2 := range fieldsInfo2.fragmentNames {
			conflicts = rule.collectConflictsBetweenFragments(conflicts, areMutuallyExclusive, fragmentName1, fragmentName2)
		}
	}
	return conflicts
}

// Collect all Conflicts "within" one collection of fields.
func (rule *overlappingFieldsCanBeMergedRule) collectConflictsWithin(conflicts []conflict, fieldsInfo *fieldsAndFragmentNames) []conflict {
	// A field map is a keyed collection, where each key represents a response
	// name and the value at that key is a list of all fields which provide that
	// response name. For every response name, if there are multiple fields, they
	// must be compared to find a potential conflict.
	for _, responseName := range fieldsInfo.fieldsOrder {
		fields, ok := fieldsInfo.fieldMap[responseName]
		if !ok {
			continue
		}
		// This compares every field in the list to every other field in this list
		// (except to itself). If the list only has one item, nothing needs to
		// be compared.
		if len(fields) <= 1 {
			continue
		}
		for i := 0; i < len(fields); i++ {
			for k := i + 1; k < len(fields); k++ {
				// within one collection is never mutually exclusive
				isMutuallyExclusive := false
				conflict := rule.findConflict(isMutuallyExclusive, responseName, fields[i], fields[k])
				if conflict != nil {
					conflicts = append(conflicts, *conflict)
				}
			}
		}
	}
	return conflicts
}

// Collect all Conflicts between two collections of fields. This is similar to,
// but different from the `collectConflictsWithin` function above. This check
// assumes that `collectConflictsWithin` has already been called on each
// provided collection of fields. This is true because this validator traverses
// each individual selection set.
func (rule *overlappingFieldsCanBeMergedRule) collectConflictsBetween(conflicts []conflict, parentFieldsAreMutuallyExclusive bool,
	fieldsInfo1 *fieldsAndFragmentNames,
	fieldsInfo2 *fieldsAndFragmentNames) []conflict {
	// A field map is a keyed collection, where each key represents a response
	// name and the value at that key is a list of all fields which provide that
	// response name. For any response name which appears in both provided field
	// maps, each field from the first field map must be compared to every field
	// in the second field map to find potential conflicts.
	for _, responseName := range fieldsInfo1.fieldsOrder {
		fields1, ok1 := fieldsInfo1.fieldMap[responseName]
		fields2, ok2 := fieldsInfo2.fieldMap[responseName]
		if !ok1 || !ok2 {
			continue
		}
		for i := 0; i < len(fields1); i++ {
			for k := 0; k < len(fields2); k++ {
				conflict := rule.findConflict(parentFieldsAreMutuallyExclusive, responseName, fields1[i], fields2[k])
				if conflict != nil {
					conflicts = append(conflicts, *conflict)
				}
			}
		}
	}
	return conflicts
}

// findConflict Determines if there is a conflict between two particular fields.
func (rule *overlappingFieldsCanBeMergedRule) findConflict(parentFieldsAreMutuallyExclusive bool, responseName string, field *fieldDefPair, field2 *fieldDefPair) *conflict {

	parentType1 := field.ParentType
	ast1 := field.Field
	def1 := field.FieldDef

	parentType2 := field2.ParentType
	ast2 := field2.Field
	def2 := field2.FieldDef

	// If it is known that two fields could not possibly apply at the same
	// time, due to the parent types, then it is safe to permit them to diverge
	// in aliased field or arguments used as they will not present any ambiguity
	// by differing.
	// It is known that two parent types could never overlap if they are
	// different Object types. Interface or Union types might overlap - if not
	// in the current state of the schema, then perhaps in some future version,
	// thus may not safely diverge.
	_, isParentType1Object := parentType1.(*Object)
	_, isParentType2Object := parentType2.(*Object)
	areMutuallyExclusive := parentFieldsAreMutuallyExclusive || parentType1 != parentType2 && isParentType1Object && isParentType2Object

	// The return type for each field.
	var type1 Type
	var type2 Type
	if def1 != nil {
		type1 = def1.Type
	}
	if def2 != nil {
		type2 = def2.Type
	}

	if !areMutuallyExclusive {
		// Two aliases must refer to the same field.
		name1 := ""
		name2 := ""

		if ast1.Name != nil {
			name1 = ast1.Name.Value
		}
		if ast2.Name != nil {
			name2 = ast2.Name.Value
		}
		if name1 != name2 {
			return &conflict{
				Reason: conflictReason{
					Name:    responseName,
					Message: fmt.Sprintf(`%v and %v are different fields`, name1, name2),
				},
				FieldsLeft:  []ast.Node{ast1},
				FieldsRight: []ast.Node{ast2},
			}
		}

		// Two field calls must have the same arguments.
		if !sameArguments(ast1.Arguments, ast2.Arguments) {
			return &conflict{
				Reason: conflictReason{
					Name:    responseName,
					Message: `they have differing arguments`,
				},
				FieldsLeft:  []ast.Node{ast1},
				FieldsRight: []ast.Node{ast2},
			}
		}
	}

	if type1 != nil && type2 != nil && doTypesConflict(type1, type2) {
		return &conflict{
			Reason: conflictReason{
				Name:    responseName,
				Message: fmt.Sprintf(`they return conflicting types %v and %v`, type1, type2),
			},
			FieldsLeft:  []ast.Node{ast1},
			FieldsRight: []ast.Node{ast2},
		}
	}

	// Collect and compare sub-fields. Use the same "visited fragment names" list
	// for both collections so fields in a fragment reference are never
	// compared to themselves.
	selectionSet1 := ast1.SelectionSet
	selectionSet2 := ast2.SelectionSet
	if selectionSet1 != nil && selectionSet2 != nil {
		conflicts := rule.findConflictsBetweenSubSelectionSets(areMutuallyExclusive, GetNamed(type1), selectionSet1, GetNamed(type2), selectionSet2)
		return subfieldConflicts(conflicts, responseName, ast1, ast2)
	}
	return nil
}

// Given a selection set, return the collection of fields (a mapping of response
// name to field ASTs and definitions) as well as a list of fragment names
// referenced via fragment spreads.
func (rule *overlappingFieldsCanBeMergedRule) getFieldsAndFragmentNames(parentType Named, selectionSet *ast.SelectionSet) *fieldsAndFragmentNames {
	if cached, ok := rule.cacheMap[selectionSet]; ok && cached != nil {
		return cached
	}

	astAndDefs := astAndDefCollection{}
	fieldsOrder := []string{}
	fragmentNames := []string{}
	fragmentNamesMap := map[string]bool{}

	var collectFieldsAndFragmentNames func(parentType Named, selectionSet *ast.SelectionSet)
	collectFieldsAndFragmentNames = func(parentType Named, selectionSet *ast.SelectionSet) {
		for _, selection := range selectionSet.Selections {
			switch selection := selection.(type) {
			case *ast.Field:
				fieldName := ""
				if selection.Name != nil {
					fieldName = selection.Name.Value
				}
				var fieldDef *FieldDefinition
				if parentType, ok := parentType.(*Object); ok && parentType != nil {
					fieldDef, _ = parentType.Fields()[fieldName]
				}
				if parentType, ok := parentType.(*Interface); ok && parentType != nil {
					fieldDef, _ = parentType.Fields()[fieldName]
				}

				responseName := fieldName
				if selection.Alias != nil {
					responseName = selection.Alias.Value
				}

				fieldDefPairs, ok := astAndDefs[responseName]
				if !ok || fieldDefPairs == nil {
					fieldDefPairs = []*fieldDefPair{}
					fieldsOrder = append(fieldsOrder, responseName)
				}

				fieldDefPairs = append(fieldDefPairs, &fieldDefPair{
					ParentType: parentType,
					Field:      selection,
					FieldDef:   fieldDef,
				})
				astAndDefs[responseName] = fieldDefPairs
			case *ast.FragmentSpread:
				fieldName := ""
				if selection.Name != nil {
					fieldName = selection.Name.Value
				}
				if val, ok := fragmentNamesMap[fieldName]; !ok || !val {
					fragmentNamesMap[fieldName] = true
					fragmentNames = append(fragmentNames, fieldName)
				}
			case *ast.InlineFragment:
				typeCondition := selection.TypeCondition
				inlineFragmentType := parentType
				if typeCondition != nil {
					ttype, err := typeFromAST(*(rule.context.Schema()), typeCondition)
					if err == nil {
						inlineFragmentType, _ = ttype.(Named)
					}
				}
				collectFieldsAndFragmentNames(inlineFragmentType, selection.SelectionSet)
			}
		}
	}
	collectFieldsAndFragmentNames(parentType, selectionSet)

	cached := &fieldsAndFragmentNames{
		fieldMap:      astAndDefs,
		fieldsOrder:   fieldsOrder,
		fragmentNames: fragmentNames,
	}

	rule.cacheMap[selectionSet] = cached
	return cached
}

func (rule *overlappingFieldsCanBeMergedRule) getReferencedFieldsAndFragmentNames(fragment *ast.FragmentDefinition) *fieldsAndFragmentNames {
	// Short-circuit building a type from the AST if possible.
	if cached, ok := rule.cacheMap[fragment.SelectionSet]; ok && cached != nil {
		return cached
	}
	fragmentType, err := typeFromAST(*(rule.context.Schema()), fragment.TypeCondition)
	if err != nil {
		return nil
	}
	return rule.getFieldsAndFragmentNames(fragmentType, fragment.SelectionSet)
}

type conflictReason struct {
	Name    string
	Message interface{} // conflictReason || []conflictReason
}
type conflict struct {
	Reason      conflictReason
	FieldsLeft  []ast.Node
	FieldsRight []ast.Node
}

// a.k.a AstAndDef
type fieldDefPair struct {
	ParentType Named
	Field      *ast.Field
	FieldDef   *FieldDefinition
}
type astAndDefCollection map[string][]*fieldDefPair

// cache struct for fields, its order and fragments names
type fieldsAndFragmentNames struct {
	fieldMap      astAndDefCollection
	fieldsOrder   []string // stores the order of field names in fieldMap
	fragmentNames []string
}

// pairSet A way to keep track of pairs of things when the ordering of the pair does
// not matter. We do this by maintaining a sort of double adjacency sets.
type pairSet struct {
	data map[string]map[string]bool
}

func newPairSet() *pairSet {
	return &pairSet{
		data: map[string]map[string]bool{},
	}
}
func (pair *pairSet) Has(a string, b string, areMutuallyExclusive bool) bool {
	first, ok := pair.data[a]
	if !ok || first == nil {
		return false
	}
	res, ok := first[b]
	if !ok {
		return false
	}
	// areMutuallyExclusive being false is a superset of being true,
	// hence if we want to know if this PairSet "has" these two with no
	// exclusivity, we have to ensure it was added as such.
	if !areMutuallyExclusive {
		return res == false
	}
	return true
}
func (pair *pairSet) Add(a string, b string, areMutuallyExclusive bool) {
	pair.data = pairSetAdd(pair.data, a, b, areMutuallyExclusive)
	pair.data = pairSetAdd(pair.data, b, a, areMutuallyExclusive)
}
func pairSetAdd(data map[string]map[string]bool, a, b string, areMutuallyExclusive bool) map[string]map[string]bool {
	set, ok := data[a]
	if !ok || set == nil {
		set = map[string]bool{}
	}
	set[b] = areMutuallyExclusive
	data[a] = set
	return data
}

func sameArguments(args1 []*ast.Argument, args2 []*ast.Argument) bool {
	if len(args1) != len(args2) {
		return false
	}

	for _, arg1 := range args1 {
		arg1Name := ""
		if arg1.Name != nil {
			arg1Name = arg1.Name.Value
		}

		var foundArgs2 *ast.Argument
		for _, arg2 := range args2 {
			arg2Name := ""
			if arg2.Name != nil {
				arg2Name = arg2.Name.Value
			}
			if arg1Name == arg2Name {
				foundArgs2 = arg2
				break
			}
		}
		if foundArgs2 == nil {
			return false
		}
		if sameValue(arg1.Value, foundArgs2.Value) == false {
			return false
		}
	}

	return true
}

func sameValue(value1 ast.Value, value2 ast.Value) bool {
	if value1 == nil && value2 == nil {
		return true
	}
	val1 := printer.Print(value1)
	val2 := printer.Print(value2)

	return val1 == val2
}

// Two types conflict if both types could not apply to a value simultaneously.
// Composite types are ignored as their individual field types will be compared
// later recursively. However List and Non-Null types must match.
func doTypesConflict(type1 Output, type2 Output) bool {
	if type1, ok := type1.(*List); ok {
		if type2, ok := type2.(*List); ok {
			return doTypesConflict(type1.OfType, type2.OfType)
		}
		return true
	}
	if type2, ok := type2.(*List); ok {
		if type1, ok := type1.(*List); ok {
			return doTypesConflict(type1.OfType, type2.OfType)
		}
		return true
	}
	if type1, ok := type1.(*NonNull); ok {
		if type2, ok := type2.(*NonNull); ok {
			return doTypesConflict(type1.OfType, type2.OfType)
		}
		return true
	}
	if type2, ok := type2.(*NonNull); ok {
		if type1, ok := type1.(*NonNull); ok {
			return doTypesConflict(type1.OfType, type2.OfType)
		}
		return true
	}
	if IsLeafType(type1) || IsLeafType(type2) {
		return type1 != type2
	}
	return false
}

// subfieldConflicts Given a series of Conflicts which occurred between two sub-fields, generate a single Conflict.
func subfieldConflicts(conflicts []conflict, responseName string, ast1 *ast.Field, ast2 *ast.Field) *conflict {
	if len(conflicts) > 0 {
		conflictReasons := []conflictReason{}
		conflictFieldsLeft := []ast.Node{ast1}
		conflictFieldsRight := []ast.Node{ast2}
		for _, c := range conflicts {
			conflictReasons = append(conflictReasons, c.Reason)
			conflictFieldsLeft = append(conflictFieldsLeft, c.FieldsLeft...)
			conflictFieldsRight = append(conflictFieldsRight, c.FieldsRight...)
		}

		return &conflict{
			Reason: conflictReason{
				Name:    responseName,
				Message: conflictReasons,
			},
			FieldsLeft:  conflictFieldsLeft,
			FieldsRight: conflictFieldsRight,
		}
	}
	return nil
}
