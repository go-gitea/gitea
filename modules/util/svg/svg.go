// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package svg

import (
	"bytes"
	"io"

	"github.com/microcosm-cc/bluemonday"

	minify "github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/svg"
)

// MinifySVG compact svg strings
func MinifySVG(svgData io.Reader) (*bytes.Buffer, error) {
	m := minify.New()
	m.AddFunc("image/svg+xml", svg.Minify)
	var out bytes.Buffer
	err := m.Minify("image/svg+xml", &out, svgData)
	return &out, err
}

// SanitizeSVG remove potential malicious dom elements
func SanitizeSVG(svgData io.Reader) *bytes.Buffer {
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
	return p.SanitizeReader(svgData)
}
