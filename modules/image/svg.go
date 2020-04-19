// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package image

import (
	"io"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// SanitizeSVG remove potential malicious dom elements
func SanitizeSVG(svgData io.Reader) string {
	//TODO init policy at start-up and keep it
	p := bluemonday.NewPolicy()
	p.AllowElements("svg", "title", "path", "desc", "g", "a")
	p.AllowNoAttrs().OnElements("svg", "title", "desc", "g", "a")
	p.AllowAttrs("id", "viewBox", "role", "aria-labelledby", "xmlns", "xmlns:xlink", "xml:space").OnElements("svg")
	p.AllowAttrs("version").Matching(regexp.MustCompile(`^\d$`)).OnElements("svg")
	p.AllowAttrs("id").OnElements("title", "desc")
	p.AllowAttrs("id", "data-name", "class", "aria-label").OnElements("g")
	p.AllowAttrs("id", "data-name", "class", "d", "transform", "aria-haspopup").OnElements("path")
	p.AllowAttrs("x", "y", "width", "height").OnElements("rect", "svg")

	p.AllowAttrs("href", "xlink:href").Matching(regexp.MustCompile(`^#\w+$`)).OnElements("a")

	//TODO find a good way to allow relative url import
	//var invalidID = regexp.MustCompile(`((http|ftp)s?)|(url *\( *' *//)`)
	//var validID = regexp.MustCompile(`(?!((http|ftp)s?)|(url *\( *' *//))`) //not supported
	//p.AllowAttrs("fill").Matching(regexp.MustCompile(`^(\w+)|(url\(#\w+\))$`)).OnElements("rect")
	p.AllowAttrs("fill").Matching(regexp.MustCompile(`^\w+$`)).OnElements("rect")

	p.SkipElementsContent("this", "script")
	cleanedSVG := p.SanitizeReader(svgData).String()

	//Remove empty lines
	cleanedSVG = strings.TrimSpace(cleanedSVG)
	r := regexp.MustCompile("\n+") //TODO move this somewhere else
	cleanedSVG = r.ReplaceAllString(cleanedSVG, "\n")

	return cleanedSVG
}
