// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package svg

import "github.com/microcosm-cc/bluemonday"

// SanitizeSVG remove potential malicious dom elements
func SanitizeSVG(svg string) string {
	p := bluemonday.NewPolicy()
	p.AllowElements("svg", "title", "path", "desc", "g")
	p.AllowAttrs("id", "viewbox", "role", "aria-labelledby").OnElements("svg")
	p.AllowAttrs("id").OnElements("title", "desc")
	p.AllowAttrs("id", "data-name", "class", "aria-label").OnElements("g")
	p.AllowAttrs("id", "data-name", "class", "d", "transform", "aria-haspopup").OnElements("path")
	return p.Sanitize(svg)
}
