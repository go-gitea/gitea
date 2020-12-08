package diff

// This is a simple DSL for diffing arrays

// fromArrayStruct utility struct to encompass diffing of string arrays
type fromArrayStruct struct {
	from []string
}

// fromStringArray starts a fluent diff expression
func fromStringArray(from []string) fromArrayStruct {
	return fromArrayStruct{from}
}

// DiffsTo completes a fluent diff expression
func (f fromArrayStruct) DiffsTo(toArray []string) (added, deleted, common []string) {
	inFrom := 1
	inTo := 2

	if f.from == nil {
		return toArray, []string{}, []string{}
	}

	m := make(map[string]int, len(toArray))
	added = make([]string, 0, len(toArray))
	deleted = make([]string, 0, len(f.from))
	common = make([]string, 0, len(f.from))

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

// fromMapStruct utility struct to encompass diffing of string arrays
type fromMapStruct struct {
	srcMap map[string]interface{}
}

// fromStringMap starts a comparison by declaring a source map
func fromStringMap(srcMap map[string]interface{}) fromMapStruct {
	return fromMapStruct{srcMap}
}

// Pair stores a pair of items which share a key in two maps
type Pair struct {
	First  interface{}
	Second interface{}
}

// DiffsTo - generates diffs for a comparison
func (f fromMapStruct) DiffsTo(destMap map[string]interface{}) (added, deleted, common map[string]interface{}) {
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
