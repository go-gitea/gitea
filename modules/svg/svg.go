// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package svg

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/html"
)

var (
	// SVGs contains discovered SVGs
	SVGs map[string]string

	widthRe  = regexp.MustCompile(`width="[0-9]+?"`)
	heightRe = regexp.MustCompile(`height="[0-9]+?"`)
)

const defaultSize = 16

// Init discovers SVGs and populates the `SVGs` variable
func Init() {
	SVGs = Discover()
}

// Render render icons - arguments icon name (string), size (int), class (string)
func RenderHTML(icon string, others ...interface{}) template.HTML {
	size, class := html.ParseSizeAndClass(defaultSize, "", others...)

	if svgStr, ok := SVGs[icon]; ok {
		if size != defaultSize {
			svgStr = widthRe.ReplaceAllString(svgStr, fmt.Sprintf(`width="%d"`, size))
			svgStr = heightRe.ReplaceAllString(svgStr, fmt.Sprintf(`height="%d"`, size))
		}
		if class != "" {
			svgStr = strings.Replace(svgStr, `class="`, fmt.Sprintf(`class="%s `, class), 1)
		}
		return template.HTML(svgStr)
	}
	return template.HTML("")
}
