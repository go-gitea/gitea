// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package svg

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"
)

// SVGs contains discovered SVGs
var SVGs map[string]string

// Init discovers SVGs and populates the `SVGs` variable
func Init() {
	SVGs = Discover()
}

var (
	widthRe  = regexp.MustCompile(`width="[0-9]+?"`)
	heightRe = regexp.MustCompile(`height="[0-9]+?"`)
)

// ParseOthers get size and class from string with default values
func ParseOthers(defaultSize int, defaultClass string, others ...interface{}) (int, string) {
	if len(others) == 0 {
		return defaultSize, defaultClass
	}

	size := defaultSize
	_size, ok := others[0].(int)
	if ok && _size != 0 {
		size = _size
	}

	if len(others) == 1 {
		return size, defaultClass
	}

	class := defaultClass
	if _class, ok := others[1].(string); ok && _class != "" {
		if defaultClass == "" {
			class = _class
		} else {
			class = defaultClass + " " + _class
		}
	}

	return size, class
}

// Render render icons - arguments icon name (string), size (int), class (string)
func RenderHTML(icon string, others ...interface{}) template.HTML {
	size, class := ParseOthers(16, "", others...)

	if svgStr, ok := SVGs[icon]; ok {
		if size != 16 {
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
