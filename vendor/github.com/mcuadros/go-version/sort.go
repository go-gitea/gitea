package version

import (
	"sort"
)

// Sorts a string slice of version number strings using version.CompareSimple()
//
// Example:
//     version.Sort([]string{"1.10-dev", "1.0rc1", "1.0", "1.0-dev"})
//     Returns []string{"1.0-dev", "1.0rc1", "1.0", "1.10-dev"}
//
func Sort(versionStrings []string) {
	versions := versionSlice(versionStrings)
	sort.Sort(versions)
}

type versionSlice []string

func (s versionSlice) Len() int {
	return len(s)
}

func (s versionSlice) Less(i, j int) bool {
	cmp := CompareSimple(Normalize(s[i]), Normalize(s[j]))
	if cmp == 0 {
		return s[i] < s[j]
	}
	return cmp < 0
}

func (s versionSlice) Swap(i, j int) {
	tmp := s[j]
	s[j] = s[i]
	s[i] = tmp
}
