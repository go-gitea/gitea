// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package svg

import (
	"fmt"
	"html/template"
	"path"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/html"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/public"
)

var (
	// SVGs contains discovered SVGs
	SVGs = map[string]string{}

	widthRe  = regexp.MustCompile(`width="[0-9]+?"`)
	heightRe = regexp.MustCompile(`height="[0-9]+?"`)
)

const defaultSize = 16

// Init discovers SVGs and populates the `SVGs` variable
func Init() error {
	files, err := public.AssetFS().ListFiles("img/svg")
	if err != nil {
		return err
	}

	// Remove `xmlns` because inline SVG does not need it
	reXmlns := regexp.MustCompile(`(<svg\b[^>]*?)\s+xmlns="[^"]*"`)
	for _, file := range files {
		if path.Ext(file) != ".svg" {
			continue
		}
		bs, err := public.AssetFS().ReadFile("img/svg", file)
		if err != nil {
			log.Error("Failed to read SVG file %s: %v", file, err)
		} else {
			SVGs[file[:len(file)-4]] = reXmlns.ReplaceAllString(string(bs), "$1")
		}
	}
	return nil
}

// RenderHTML renders icons - arguments icon name (string), size (int), class (string)
func RenderHTML(icon string, others ...any) template.HTML {
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
	return ""
}
