package enry

import (
	"bytes"
	"path/filepath"
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

// IsVendor returns whether or not path is a vendor path.
func IsVendor(path string) bool {
	return matchRegexSlice(data.VendorMatchers, path)
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
