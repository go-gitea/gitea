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
	p := bluemonday.NewPolicy()
	p.AllowElements("svg", "title", "path", "desc", "g")
	p.AllowAttrs("id", "viewbox", "role", "aria-labelledby", "xmlns", "xmlns:xlink", "xml:space").OnElements("svg")
	p.AllowAttrs("id").OnElements("title", "desc")
	p.AllowAttrs("id", "data-name", "class", "aria-label").OnElements("g")
	p.AllowAttrs("id", "data-name", "class", "d", "transform", "aria-haspopup").OnElements("path")
	p.AllowAttrs("x", "y", "width", "height").OnElements("rect")

	//var invalidID = regexp.MustCompile(`((http|ftp)s?)|(url *\( *' *//)`)
	//var validID = regexp.MustCompile(`(?!((http|ftp)s?)|(url *\( *' *//))`) //not supported
	//p.AllowAttrs("fill").Matching(regexp.MustCompile(`((http|ftp)s?)|(url *\( *' *//)`)).OnElements("rect") //TODO match opposite

	p.SkipElementsContent("this", "script")
	cleanedSVG := p.SanitizeReader(svgData).String()

	//Remove empty lines
	cleanedSVG = strings.TrimSpace(cleanedSVG)
	r := regexp.MustCompile("\n+") //TODO move this somewhere else
	cleanedSVG = r.ReplaceAllString(cleanedSVG, "\n")

	return cleanedSVG
}
