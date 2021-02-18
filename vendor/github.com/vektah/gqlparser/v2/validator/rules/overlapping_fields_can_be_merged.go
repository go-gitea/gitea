package validator

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/vektah/gqlparser/v2/ast"
	. "github.com/vektah/gqlparser/v2/validator"
)

func init() {

	AddRule("OverlappingFieldsCanBeMerged", func(observers *Events, addError AddErrFunc) {
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

		m := &overlappingFieldsCanBeMergedManager{
			comparedFragmentPairs: pairSet{data: make(map[string]map[string]bool)},
		}

		observers.OnOperation(func(walker *Walker, operation *ast.OperationDefinition) {
			m.walker = walker
			conflicts := m.findConflictsWithinSelectionSet(operation.SelectionSet)
			for _, conflict := range conflicts {
				conflict.addFieldsConflictMessage(addError)
			}
		})
		observers.OnField(func(walker *Walker, field *ast.Field) {
			if walker.CurrentOperation == nil {
				// When checking both Operation and Fragment, errors are duplicated when processing FragmentDefinition referenced from Operation
				return
			}
			m.walker = walker
			conflicts := m.findConflictsWithinSelectionSet(field.SelectionSet)
			for _, conflict := range conflicts {
				conflict.addFieldsConflictMessage(addError)
			}
		})
		observers.OnInlineFragment(func(walker *Walker, inlineFragment *ast.InlineFragment) {
			m.walker = walker
			conflicts := m.findConflictsWithinSelectionSet(inlineFragment.SelectionSet)
			for _, conflict := range conflicts {
				conflict.addFieldsConflictMessage(addError)
			}
		})
		observers.OnFragment(func(walker *Walker, fragment *ast.FragmentDefinition) {
			m.walker = walker
			conflicts := m.findConflictsWithinSelectionSet(fragment.SelectionSet)
			for _, conflict := range conflicts {
				conflict.addFieldsConflictMessage(addError)
			}
		})
	})
}

type pairSet struct {
	data map[string]map[string]bool
}

func (pairSet *pairSet) Add(a *ast.FragmentSpread, b *ast.FragmentSpread, areMutuallyExclusive bool) {
	add := func(a *ast.FragmentSpread, b *ast.FragmentSpread) {
		m := pairSet.data[a.Name]
		if m == nil {
			m = make(map[string]bool)
			pairSet.data[a.Name] = m
		}
		m[b.Name] = areMutuallyExclusive
	}
	add(a, b)
	add(b, a)
}

func (pairSet *pairSet) Has(a *ast.FragmentSpread, b *ast.FragmentSpread, areMutuallyExclusive bool) bool {
	am, ok := pairSet.data[a.Name]
	if !ok {
		return false
	}
	result, ok := am[b.Name]
	if !ok {
		return false
	}

	// areMutuallyExclusive being false is a superset of being true,
	// hence if we want to know if this PairSet "has" these two with no
	// exclusivity, we have to ensure it was added as such.
	if !areMutuallyExclusive {
		return !result
	}

	return true
}

type sequentialFieldsMap struct {
	// We can't use map[string][]*ast.Field. because map is not stable...
	seq  []string
	data map[string][]*ast.Field
}

type fieldIterateEntry struct {
	ResponseName string
	Fields       []*ast.Field
}

func (m *sequentialFieldsMap) Push(responseName string, field *ast.Field) {
	fields, ok := m.data[responseName]
	if !ok {
		m.seq = append(m.seq, responseName)
	}
	fields = append(fields, field)
	m.data[responseName] = fields
}

func (m *sequentialFieldsMap) Get(responseName string) ([]*ast.Field, bool) {
	fields, ok := m.data[responseName]
	return fields, ok
}

func (m *sequentialFieldsMap) Iterator() [][]*ast.Field {
	fieldsList := make([][]*ast.Field, 0, len(m.seq))
	for _, responseName := range m.seq {
		fields := m.data[responseName]
		fieldsList = append(fieldsList, fields)
	}
	return fieldsList
}

func (m *sequentialFieldsMap) KeyValueIterator() []*fieldIterateEntry {
	fieldEntriesList := make([]*fieldIterateEntry, 0, len(m.seq))
	for _, responseName := range m.seq {
		fields := m.data[responseName]
		fieldEntriesList = append(fieldEntriesList, &fieldIterateEntry{
			ResponseName: responseName,
			Fields:       fields,
		})
	}
	return fieldEntriesList
}

type conflictMessageContainer struct {
	Conflicts []*ConflictMessage
}

type ConflictMessage struct {
	Message      string
	ResponseName string
	Names        []string
	SubMessage   []*ConflictMessage
	Position     *ast.Position
}

func (m *ConflictMessage) String(buf *bytes.Buffer) {
	if len(m.SubMessage) == 0 {
		buf.WriteString(m.Message)
		return
	}

	for idx, subMessage := range m.SubMessage {
		buf.WriteString(`subfields "`)
		buf.WriteString(subMessage.ResponseName)
		buf.WriteString(`" conflict because `)
		subMessage.String(buf)
		if idx != len(m.SubMessage)-1 {
			buf.WriteString(" and ")
		}
	}
}

func (m *ConflictMessage) addFieldsConflictMessage(addError AddErrFunc) {
	var buf bytes.Buffer
	m.String(&buf)
	addError(
		Message(`Fields "%s" conflict because %s. Use different aliases on the fields to fetch both if this was intentional.`, m.ResponseName, buf.String()),
		At(m.Position),
	)
}

type overlappingFieldsCanBeMergedManager struct {
	walker *Walker

	// per walker
	comparedFragmentPairs pairSet
	// cachedFieldsAndFragmentNames interface{}

	// per selectionSet
	comparedFragments map[string]bool
}

func (m *overlappingFieldsCanBeMergedManager) findConflictsWithinSelectionSet(selectionSet ast.SelectionSet) []*ConflictMessage {
	if len(selectionSet) == 0 {
		return nil
	}

	fieldsMap, fragmentSpreads := getFieldsAndFragmentNames(selectionSet)

	var conflicts conflictMessageContainer

	// (A) Find find all conflicts "within" the fieldMap of this selection set.
	// Note: this is the *only place* `collectConflictsWithin` is called.
	m.collectConflictsWithin(&conflicts, fieldsMap)

	m.comparedFragments = make(map[string]bool)
	for idx, fragmentSpreadA := range fragmentSpreads {
		// (B) Then collect conflicts between these fieldMap and those represented by
		// each spread fragment name found.
		m.collectConflictsBetweenFieldsAndFragment(&conflicts, false, fieldsMap, fragmentSpreadA)

		for _, fragmentSpreadB := range fragmentSpreads[idx+1:] {
			// (C) Then compare this fragment with all other fragments found in this
			// selection set to collect conflicts between fragments spread together.
			// This compares each item in the list of fragment names to every other
			// item in that same list (except for itself).
			m.collectConflictsBetweenFragments(&conflicts, false, fragmentSpreadA, fragmentSpreadB)
		}
	}

	return conflicts.Conflicts
}

func (m *overlappingFieldsCanBeMergedManager) collectConflictsBetweenFieldsAndFragment(conflicts *conflictMessageContainer, areMutuallyExclusive bool, fieldsMap *sequentialFieldsMap, fragmentSpread *ast.FragmentSpread) {
	if m.comparedFragments[fragmentSpread.Name] {
		return
	}
	m.comparedFragments[fragmentSpread.Name] = true

	if fragmentSpread.Definition == nil {
		return
	}

	fieldsMapB, fragmentSpreads := getFieldsAndFragmentNames(fragmentSpread.Definition.SelectionSet)

	// Do not compare a fragment's fieldMap to itself.
	if reflect.DeepEqual(fieldsMap, fieldsMapB) {
		return
	}

	// (D) First collect any conflicts between the provided collection of fields
	// and the collection of fields represented by the given fragment.
	m.collectConflictsBetween(conflicts, areMutuallyExclusive, fieldsMap, fieldsMapB)

	// (E) Then collect any conflicts between the provided collection of fields
	// and any fragment names found in the given fragment.
	baseFragmentSpread := fragmentSpread
	for _, fragmentSpread := range fragmentSpreads {
		if fragmentSpread.Name == baseFragmentSpread.Name {
			continue
		}
		m.collectConflictsBetweenFieldsAndFragment(conflicts, areMutuallyExclusive, fieldsMap, fragmentSpread)
	}
}

func (m *overlappingFieldsCanBeMergedManager) collectConflictsBetweenFragments(conflicts *conflictMessageContainer, areMutuallyExclusive bool, fragmentSpreadA *ast.FragmentSpread, fragmentSpreadB *ast.FragmentSpread) {

	var check func(fragmentSpreadA *ast.FragmentSpread, fragmentSpreadB *ast.FragmentSpread)
	check = func(fragmentSpreadA *ast.FragmentSpread, fragmentSpreadB *ast.FragmentSpread) {

		if fragmentSpreadA.Name == fragmentSpreadB.Name {
			return
		}

		if m.comparedFragmentPairs.Has(fragmentSpreadA, fragmentSpreadB, areMutuallyExclusive) {
			return
		}
		m.comparedFragmentPairs.Add(fragmentSpreadA, fragmentSpreadB, areMutuallyExclusive)

		if fragmentSpreadA.Definition == nil {
			return
		}
		if fragmentSpreadB.Definition == nil {
			return
		}

		fieldsMapA, fragmentSpreadsA := getFieldsAndFragmentNames(fragmentSpreadA.Definition.SelectionSet)
		fieldsMapB, fragmentSpreadsB := getFieldsAndFragmentNames(fragmentSpreadB.Definition.SelectionSet)

		// (F) First, collect all conflicts between these two collections of fields
		// (not including any nested fragments).
		m.collectConflictsBetween(conflicts, areMutuallyExclusive, fieldsMapA, fieldsMapB)

		// (G) Then collect conflicts between the first fragment and any nested
		// fragments spread in the second fragment.
		for _, fragmentSpread := range fragmentSpreadsB {
			check(fragmentSpreadA, fragmentSpread)
		}
		// (G) Then collect conflicts between the second fragment and any nested
		// fragments spread in the first fragment.
		for _, fragmentSpread := range fragmentSpreadsA {
			check(fragmentSpread, fragmentSpreadB)
		}
	}

	check(fragmentSpreadA, fragmentSpreadB)
}

func (m *overlappingFieldsCanBeMergedManager) findConflictsBetweenSubSelectionSets(areMutuallyExclusive bool, selectionSetA ast.SelectionSet, selectionSetB ast.SelectionSet) *conflictMessageContainer {
	var conflicts conflictMessageContainer

	fieldsMapA, fragmentSpreadsA := getFieldsAndFragmentNames(selectionSetA)
	fieldsMapB, fragmentSpreadsB := getFieldsAndFragmentNames(selectionSetB)

	// (H) First, collect all conflicts between these two collections of field.
	m.collectConflictsBetween(&conflicts, areMutuallyExclusive, fieldsMapA, fieldsMapB)

	// (I) Then collect conflicts between the first collection of fields and
	// those referenced by each fragment name associated with the second.
	for _, fragmentSpread := range fragmentSpreadsB {
		m.comparedFragments = make(map[string]bool)
		m.collectConflictsBetweenFieldsAndFragment(&conflicts, areMutuallyExclusive, fieldsMapA, fragmentSpread)
	}

	// (I) Then collect conflicts between the second collection of fields and
	// those referenced by each fragment name associated with the first.
	for _, fragmentSpread := range fragmentSpreadsA {
		m.comparedFragments = make(map[string]bool)
		m.collectConflictsBetweenFieldsAndFragment(&conflicts, areMutuallyExclusive, fieldsMapB, fragmentSpread)
	}

	// (J) Also collect conflicts between any fragment names by the first and
	// fragment names by the second. This compares each item in the first set of
	// names to each item in the second set of names.
	for _, fragmentSpreadA := range fragmentSpreadsA {
		for _, fragmentSpreadB := range fragmentSpreadsB {
			m.collectConflictsBetweenFragments(&conflicts, areMutuallyExclusive, fragmentSpreadA, fragmentSpreadB)
		}
	}

	if len(conflicts.Conflicts) == 0 {
		return nil
	}

	return &conflicts
}

func (m *overlappingFieldsCanBeMergedManager) collectConflictsWithin(conflicts *conflictMessageContainer, fieldsMap *sequentialFieldsMap) {
	for _, fields := range fieldsMap.Iterator() {
		for idx, fieldA := range fields {
			for _, fieldB := range fields[idx+1:] {
				conflict := m.findConflict(false, fieldA, fieldB)
				if conflict != nil {
					conflicts.Conflicts = append(conflicts.Conflicts, conflict)
				}
			}
		}
	}
}

func (m *overlappingFieldsCanBeMergedManager) collectConflictsBetween(conflicts *conflictMessageContainer, parentFieldsAreMutuallyExclusive bool, fieldsMapA *sequentialFieldsMap, fieldsMapB *sequentialFieldsMap) {
	for _, fieldsEntryA := range fieldsMapA.KeyValueIterator() {
		fieldsB, ok := fieldsMapB.Get(fieldsEntryA.ResponseName)
		if !ok {
			continue
		}
		for _, fieldA := range fieldsEntryA.Fields {
			for _, fieldB := range fieldsB {
				conflict := m.findConflict(parentFieldsAreMutuallyExclusive, fieldA, fieldB)
				if conflict != nil {
					conflicts.Conflicts = append(conflicts.Conflicts, conflict)
				}
			}
		}
	}
}

func (m *overlappingFieldsCanBeMergedManager) findConflict(parentFieldsAreMutuallyExclusive bool, fieldA *ast.Field, fieldB *ast.Field) *ConflictMessage {
	if fieldA.Definition == nil || fieldA.ObjectDefinition == nil || fieldB.Definition == nil || fieldB.ObjectDefinition == nil {
		return nil
	}

	areMutuallyExclusive := parentFieldsAreMutuallyExclusive
	if !areMutuallyExclusive {
		tmp := fieldA.ObjectDefinition.Name != fieldB.ObjectDefinition.Name
		tmp = tmp && fieldA.ObjectDefinition.Kind == ast.Object
		tmp = tmp && fieldB.ObjectDefinition.Kind == ast.Object
		areMutuallyExclusive = tmp
	}

	fieldNameA := fieldA.Name
	if fieldA.Alias != "" {
		fieldNameA = fieldA.Alias
	}

	if !areMutuallyExclusive {
		// Two aliases must refer to the same field.
		if fieldA.Name != fieldB.Name {
			return &ConflictMessage{
				ResponseName: fieldNameA,
				Message:      fmt.Sprintf(`%s and %s are different fields`, fieldA.Name, fieldB.Name),
				Position:     fieldB.Position,
			}
		}

		// Two field calls must have the same arguments.
		if !sameArguments(fieldA.Arguments, fieldB.Arguments) {
			return &ConflictMessage{
				ResponseName: fieldNameA,
				Message:      "they have differing arguments",
				Position:     fieldB.Position,
			}
		}
	}

	if doTypesConflict(m.walker, fieldA.Definition.Type, fieldB.Definition.Type) {
		return &ConflictMessage{
			ResponseName: fieldNameA,
			Message:      fmt.Sprintf(`they return conflicting types %s and %s`, fieldA.Definition.Type.String(), fieldB.Definition.Type.String()),
			Position:     fieldB.Position,
		}
	}

	// Collect and compare sub-fields. Use the same "visited fragment names" list
	// for both collections so fields in a fragment reference are never
	// compared to themselves.
	conflicts := m.findConflictsBetweenSubSelectionSets(areMutuallyExclusive, fieldA.SelectionSet, fieldB.SelectionSet)
	if conflicts == nil {
		return nil
	}
	return &ConflictMessage{
		ResponseName: fieldNameA,
		SubMessage:   conflicts.Conflicts,
		Position:     fieldB.Position,
	}
}

func sameArguments(args1 []*ast.Argument, args2 []*ast.Argument) bool {
	if len(args1) != len(args2) {
		return false
	}
	for _, arg1 := range args1 {
		for _, arg2 := range args2 {
			if arg1.Name != arg2.Name {
				return false
			}
			if !sameValue(arg1.Value, arg2.Value) {
				return false
			}
		}
	}
	return true
}

func sameValue(value1 *ast.Value, value2 *ast.Value) bool {
	if value1.Kind != value2.Kind {
		return false
	}
	if value1.Raw != value2.Raw {
		return false
	}
	return true
}

func doTypesConflict(walker *Walker, type1 *ast.Type, type2 *ast.Type) bool {
	if type1.Elem != nil {
		if type2.Elem != nil {
			return doTypesConflict(walker, type1.Elem, type2.Elem)
		}
		return true
	}
	if type2.Elem != nil {
		return true
	}
	if type1.NonNull && !type2.NonNull {
		return true
	}
	if !type1.NonNull && type2.NonNull {
		return true
	}

	t1 := walker.Schema.Types[type1.NamedType]
	t2 := walker.Schema.Types[type2.NamedType]
	if (t1.Kind == ast.Scalar || t1.Kind == ast.Enum) && (t2.Kind == ast.Scalar || t2.Kind == ast.Enum) {
		return t1.Name != t2.Name
	}

	return false
}

func getFieldsAndFragmentNames(selectionSet ast.SelectionSet) (*sequentialFieldsMap, []*ast.FragmentSpread) {
	fieldsMap := sequentialFieldsMap{
		data: make(map[string][]*ast.Field),
	}
	var fragmentSpreads []*ast.FragmentSpread

	var walk func(selectionSet ast.SelectionSet)
	walk = func(selectionSet ast.SelectionSet) {
		for _, selection := range selectionSet {
			switch selection := selection.(type) {
			case *ast.Field:
				responseName := selection.Name
				if selection.Alias != "" {
					responseName = selection.Alias
				}
				fieldsMap.Push(responseName, selection)

			case *ast.InlineFragment:
				walk(selection.SelectionSet)

			case *ast.FragmentSpread:
				fragmentSpreads = append(fragmentSpreads, selection)
			}
		}
	}
	walk(selectionSet)

	return &fieldsMap, fragmentSpreads
}
