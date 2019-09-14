package diff

// This is a simple DSL for diffing arrays

// FromArrayStruct utility struct to encompass diffing of string arrays
type FromArrayStruct struct {
	from []string
}

// FromStringArray starts a fluent diff expression
func FromStringArray(from []string) FromArrayStruct {
	return FromArrayStruct{from}
}

// DiffsTo completes a fluent dff expression
func (f FromArrayStruct) DiffsTo(toArray []string) (added, deleted, common []string) {
	inFrom := 1
	inTo := 2

	m := make(map[string]int)

	for _, item := range f.from {
		m[item] = inFrom
	}

	for _, item := range toArray {
		if _, ok := m[item]; ok {
			m[item] |= inTo
		} else {
			m[item] = inTo
		}
	}
	for key, val := range m {
		switch val {
		case inFrom:
			deleted = append(deleted, key)
		case inTo:
			added = append(added, key)
		default:
			common = append(common, key)
		}
	}
	return
}

// FromMapStruct utility struct to encompass diffing of string arrays
type FromMapStruct struct {
	srcMap map[string]interface{}
}

// FromStringMap starts a comparison by declaring a source map
func FromStringMap(srcMap map[string]interface{}) FromMapStruct {
	return FromMapStruct{srcMap}
}

// Pair stores a pair of items which share a key in two maps
type Pair struct {
	First  interface{}
	Second interface{}
}

// DiffsTo - generates diffs for a comparison
func (f FromMapStruct) DiffsTo(destMap map[string]interface{}) (added, deleted, common map[string]interface{}) {
	added = make(map[string]interface{})
	deleted = make(map[string]interface{})
	common = make(map[string]interface{})

	inSrc := 1
	inDest := 2

	m := make(map[string]int)

	// enter values for all items in the source array
	for key := range f.srcMap {
		m[key] = inSrc
	}

	// now either set or 'boolean or' a new flag if in the second collection
	for key := range destMap {
		if _, ok := m[key]; ok {
			m[key] |= inDest
		} else {
			m[key] = inDest
		}
	}
	// finally inspect the values and generate the left,right and shared collections
	// for the shared items, store both values in case there's a diff
	for key, val := range m {
		switch val {
		case inSrc:
			deleted[key] = f.srcMap[key]
		case inDest:
			added[key] = destMap[key]
		default:
			common[key] = Pair{f.srcMap[key], destMap[key]}
		}
	}
	return added, deleted, common
}
