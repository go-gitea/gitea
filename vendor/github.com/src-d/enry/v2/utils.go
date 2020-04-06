package enry

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/src-d/enry/v2/data"
)

const binSniffLen = 8000

var configurationLanguages = map[string]bool{
	"XML": true, "JSON": true, "TOML": true, "YAML": true, "INI": true, "SQL": true,
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
	return data.DocumentationMatchers.Match(path)
}

// IsDotFile returns whether or not path has dot as a prefix.
func IsDotFile(path string) bool {
	base := filepath.Base(filepath.Clean(path))
	return strings.HasPrefix(base, ".") && base != "."
}

// IsVendor returns whether or not path is a vendor path.
func IsVendor(path string) bool {
	return data.VendorMatchers.Match(path)
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

	return "#cccccc"
}
