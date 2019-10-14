package version

import (
	"regexp"
	"strconv"
	"strings"
)

var regexpSigns = regexp.MustCompile(`[_\-+]`)
var regexpDotBeforeDigit = regexp.MustCompile(`([^.\d]+)`)
var regexpMultipleDots = regexp.MustCompile(`\.{2,}`)

var specialForms = map[string]int{
	"SNAPSHOT": -7,
	"snapshot": -7,
	"dev":      -6,
	"alpha":    -5,
	"a":        -5,
	"beta":     -4,
	"b":        -4,
	"RC":       -3,
	"rc":       -3,
	"#":        -2,
	"p":        1,
	"pl":       1,
}

var unknownForm int = -7

// Compare compares two version number strings, for a particular relationship
//
// Usage
//     version.Compare("2.3.4", "v3.1.2", "<")
//     Returns: true
//
//     version.Compare("1.0rc1", "1.0", ">=")
//     Returns: false
func Compare(version1, version2, operator string) bool {
	version1N := Normalize(version1)
	version2N := Normalize(version2)

	return CompareNormalized(version1N, version2N, operator)
}

// CompareNormalized compares two normalizated version number strings, for a particular relationship
//
// The function first replaces _, - and + with a dot . in the version strings
// and also inserts dots . before and after any non number so that for example
// '4.3.2RC1' becomes '4.3.2.RC.1'.
//
// Then it splits the results like if you were using Split(version, '.').
// Then it compares the parts starting from left to right. If a part contains
// special version strings these are handled in the following order: any string
// not found in this list:
//   < dev < alpha = a < beta = b < RC = rc < # < pl = p.
//
// Usage
//     version.CompareNormalized("1.0-dev", "1.0", "<")
//     Returns: true
//
//     version.CompareNormalized("1.0rc1", "1.0", ">=")
//     Returns: false
//
//     version.CompareNormalized("1.0", "1.0b1", "ge")
//     Returns: true
func CompareNormalized(version1, version2, operator string) bool {
	compare := CompareSimple(version1, version2)

	switch {
	case operator == ">" || operator == "gt":
		return compare > 0
	case operator == ">=" || operator == "ge":
		return compare >= 0
	case operator == "<=" || operator == "le":
		return compare <= 0
	case operator == "==" || operator == "=" || operator == "eq":
		return compare == 0
	case operator == "<>" || operator == "!=" || operator == "ne":
		return compare != 0
	case operator == "" || operator == "<" || operator == "lt":
		return compare < 0
	}

	return false
}

// CompareSimple compares two normalizated version number strings
//
// Just the same of CompareVersion but return a int result, 0 if both version
// are equal, 1 if the right side is bigger and -1 if the right side is lower
//
// Usage
//     version.CompareSimple("1.2", "1.0.1")
//     Returns: 1
//
//     version.CompareSimple("1.0rc1", "1.0")
//     Returns: -1
func CompareSimple(version1, version2 string) int {
	var x, r, l int = 0, 0, 0

	v1, v2 := prepVersion(version1), prepVersion(version2)
	len1, len2 := len(v1), len(v2)

	if len1 > len2 {
		x = len1
	} else {
		x = len2
	}

	for i := 0; i < x; i++ {
		if i < len1 && i < len2 {
			if v1[i] == v2[i] {
				continue
			}
		}

		r = 0
		if i < len1 {
			r = numVersion(v1[i])
		}

		l = 0
		if i < len2 {
			l = numVersion(v2[i])
		}

		if r < l {
			return -1
		} else if r > l {
			return 1
		}
	}

	return 0
}

func prepVersion(version string) []string {
	if len(version) == 0 {
		return []string{""}
	}

	version = regexpSigns.ReplaceAllString(version, ".")
	version = regexpDotBeforeDigit.ReplaceAllString(version, ".$1.")
	version = regexpMultipleDots.ReplaceAllString(version, ".")

	return strings.Split(version, ".")
}

func numVersion(value string) int {
	if value == "" {
		return 0
	}

	if number, err := strconv.Atoi(value); err == nil {
		return number
	}

	if special, ok := specialForms[value]; ok {
		return special
	}

	return unknownForm
}

func ValidSimpleVersionFormat(value string) bool {
	normalized := Normalize(value)
	for _, component := range prepVersion(normalized) {
		if numVersion(component) == unknownForm {
			return false
		}
	}
	return true
}
