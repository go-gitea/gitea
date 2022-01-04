package enry

import (
	"bytes"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/go-enry/go-enry/v2/data"
	"github.com/go-enry/go-enry/v2/regex"
)

const binSniffLen = 8000

var configurationLanguages = map[string]struct{}{
	"XML":  {},
	"JSON": {},
	"TOML": {},
	"YAML": {},
	"INI":  {},
	"SQL":  {},
}

// IsConfiguration tells if filename is in one of the configuration languages.
func IsConfiguration(path string) bool {
	language, _ := GetLanguageByExtension(path)
	_, is := configurationLanguages[language]
	return is
}

// IsImage tells if a given file is an image (PNG, JPEG or GIF format).
func IsImage(path string) bool {
	extension := filepath.Ext(path)
	if extension == ".png" || extension == ".jpg" || extension == ".jpeg" || extension == ".gif" {
		return true
	}

	return false
}

// GetMIMEType returns a MIME type of a given file based on its languages.
func GetMIMEType(path string, language string) string {
	if mime, ok := data.LanguagesMime[language]; ok {
		return mime
	}

	if IsImage(path) {
		return "image/" + filepath.Ext(path)[1:]
	}

	return "text/plain"
}

// IsDocumentation returns whether or not path is a documentation path.
func IsDocumentation(path string) bool {
	return matchRegexSlice(data.DocumentationMatchers, path)
}

// IsDotFile returns whether or not path has dot as a prefix.
func IsDotFile(path string) bool {
	base := filepath.Base(filepath.Clean(path))
	return strings.HasPrefix(base, ".") && base != "."
}

var isVendorRegExp *regexp.Regexp

// IsVendor returns whether or not path is a vendor path.
func IsVendor(path string) bool {
	return isVendorRegExp.MatchString(path)
}

// IsTest returns whether or not path is a test path.
func IsTest(path string) bool {
	return matchRegexSlice(data.TestMatchers, path)
}

// IsBinary detects if data is a binary value based on:
// http://git.kernel.org/cgit/git/git.git/tree/xdiff-interface.c?id=HEAD#n198
func IsBinary(data []byte) bool {
	if len(data) > binSniffLen {
		data = data[:binSniffLen]
	}

	if bytes.IndexByte(data, byte(0)) == -1 {
		return false
	}

	return true
}

// GetColor returns a HTML color code of a given language.
func GetColor(language string) string {
	if color, ok := data.LanguagesColor[language]; ok {
		return color
	}

	if color, ok := data.LanguagesColor[GetLanguageGroup(language)]; ok {
		return color
	}

	return "#cccccc"
}

func matchRegexSlice(exprs []regex.EnryRegexp, str string) bool {
	for _, expr := range exprs {
		if expr.MatchString(str) {
			return true
		}
	}

	return false
}

// IsGenerated returns whether the file with the given path and content is a
// generated file.
func IsGenerated(path string, content []byte) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if _, ok := data.GeneratedCodeExtensions[ext]; ok {
		return true
	}

	for _, m := range data.GeneratedCodeNameMatchers {
		if m(path) {
			return true
		}
	}

	path = strings.ToLower(path)
	for _, m := range data.GeneratedCodeMatchers {
		if m(path, ext, content) {
			return true
		}
	}

	return false
}

func init() {
	// We now collate the individual regexps that make up the VendorMatchers to
	// produce a single large regexp which is around twice as fast to test than
	// simply iterating through all the regexps or naïvely collating the
	// regexps.
	//
	// ---
	//
	// data.VendorMatchers here is a slice containing individual regexps that
	// match a vendor file therefore if we want to test if a filename is a
	// Vendor we need to test whether that filename matches one or more of
	// those regexps.
	//
	// Now we could test each matcher in turn using a shortcircuiting test i.e.
	//
	//  	func IsVendor(filename string) bool {
	// 			for _, matcher := range data.VendorMatchers {
	// 				if matcher.Match(filename) {
	//					return true
	//				}
	//			}
	//			return false
	//		}
	//
	// Or concatentate all these regexps using groups i.e.
	//
	//		`(regexp1)|(regexp2)|(regexp3)|...`
	//
	// However both of these are relatively slow and they don't take advantage
	// of the inherent structure within our regexps...
	//
	// If we look at our regexps there are essentially three types of regexp:
	//
	// 1. Those that start with `^`
	// 2. Those that start with `(^|/)`
	// 3. Others
	//
	// If we collate our regexps into these groups that will significantly
	// reduce the likelihood of backtracking within the regexp trie matcher.
	//
	// A further improvement is to use non-capturing groups as otherwise the
	// regexp parser, whilst matching, will have to allocate slices for
	// matching positions. (A future improvement here could be in the use of
	// enforcing non-capturing groups within the sub-regexps too.)
	//
	// Finally if we sort the segments we can help the matcher build a more
	// efficient matcher and trie.

	// alias the VendorMatchers to simplify things
	matchers := data.VendorMatchers

	// Create three temporary string slices for our three groups above - prefixes removed
	caretStrings := make([]string, 0, 10)
	caretSegmentStrings := make([]string, 0, 10)
	matcherStrings := make([]string, 0, len(matchers))

	// Walk the matchers and check their string representation for each group prefix, remove it and add to the respective group slices
	for _, matcher := range matchers {
		str := matcher.String()
		if str[0] == '^' {
			caretStrings = append(caretStrings, str[1:])
		} else if str[0:5] == "(^|/)" {
			caretSegmentStrings = append(caretSegmentStrings, str[5:])
		} else {
			matcherStrings = append(matcherStrings, str)
		}
	}

	// Sort the strings within each group - a potential further improvement could be in simplifying within these groups
	sort.Strings(caretSegmentStrings)
	sort.Strings(caretStrings)
	sort.Strings(matcherStrings)

	// Now build the collated regexp
	sb := &strings.Builder{}

	// Start with group 1 - those that started with `^`
	sb.WriteString("(?:^(?:")
	sb.WriteString(caretStrings[0])
	for _, matcher := range caretStrings[1:] {
		sb.WriteString(")|(?:")
		sb.WriteString(matcher)
	}
	sb.WriteString("))")
	sb.WriteString("|")

	// Now add group 2 - those that started with `(^|/)`
	sb.WriteString("(?:(?:^|/)(?:")
	sb.WriteString(caretSegmentStrings[0])
	for _, matcher := range caretSegmentStrings[1:] {
		sb.WriteString(")|(?:")
		sb.WriteString(matcher)
	}
	sb.WriteString("))")
	sb.WriteString("|")

	// Finally add the rest
	sb.WriteString("(?:")
	sb.WriteString(matcherStrings[0])
	for _, matcher := range matcherStrings[1:] {
		sb.WriteString(")|(?:")
		sb.WriteString(matcher)
	}
	sb.WriteString(")")

	// Compile the whole thing as the isVendorRegExp
	isVendorRegExp = regexp.MustCompile(sb.String())
}
