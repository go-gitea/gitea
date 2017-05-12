// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"path/filepath"
	"strings"
)

// Parser defines an interface for parsering markup file to HTML
type Parser interface {
	Name() string // markup format name
	Extensions() []string
	Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte
}

var (
	parsers = make(map[string]Parser)
)

// RegisterParser registers a new markup file parser
func RegisterParser(parser Parser) {
	for _, ext := range parser.Extensions() {
		parsers[strings.ToLower(ext)] = parser
	}
}

// Render renders markup file to HTML with all specific handling stuff.
func Render(filename string, rawBytes []byte, urlPrefix string, metas map[string]string) []byte {
	return render(filename, rawBytes, urlPrefix, metas, false)
}

func render(filename string, rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	extension := strings.ToLower(filepath.Ext(filename))
	if parser, ok := parsers[extension]; ok {
		return parser.Render(rawBytes, urlPrefix, metas, isWiki)
	}
	return nil
}

// RenderString renders Markdown to HTML with special links and returns string type.
func RenderString(filename string, raw, urlPrefix string, metas map[string]string) string {
	return string(render(filename, []byte(raw), urlPrefix, metas, false))
}

// RenderWiki renders markdown wiki page to HTML and return HTML string
func RenderWiki(filename string, rawBytes []byte, urlPrefix string, metas map[string]string) string {
	return string(render(filename, rawBytes, urlPrefix, metas, true))
}

// Type returns if markup format via the filename
func Type(filename string) string {
	extension := strings.ToLower(filepath.Ext(filename))
	if parser, ok := parsers[extension]; ok {
		return parser.Name()
	}
	return ""
}

// ReadmeFileType reports whether name looks like a README file
// based on its name and find the parser via its ext name
func ReadmeFileType(name string) (string, bool) {
	if IsReadmeFile(name) {
		return Type(name), true
	}
	return "", false
}

// IsReadmeFile reports whether name looks like a README file
// based on its name.
func IsReadmeFile(name string) bool {
	if len(name) < 6 {
		return false
	}

	name = strings.ToLower(name)
	if len(name) == 6 {
		return name == "readme"
	}
	return name[:7] == "readme."
}
