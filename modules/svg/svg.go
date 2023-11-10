// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package svg

import (
	"fmt"
	"html/template"
	"path"
	"strings"

	gitea_html "code.gitea.io/gitea/modules/html"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/public"
)

var svgIcons map[string]string

const defaultSize = 16

// Init discovers SVG icons and populates the `svgIcons` variable
func Init() error {
	const svgAssetsPath = "assets/img/svg"
	files, err := public.AssetFS().ListFiles(svgAssetsPath)
	if err != nil {
		return err
	}

	svgIcons = make(map[string]string, len(files))
	for _, file := range files {
		if path.Ext(file) != ".svg" {
			continue
		}
		bs, err := public.AssetFS().ReadFile(svgAssetsPath, file)
		if err != nil {
			log.Error("Failed to read SVG file %s: %v", file, err)
		} else {
			svgIcons[file[:len(file)-4]] = string(Normalize(bs, defaultSize))
		}
	}
	return nil
}

// RenderHTML renders icons - arguments icon name (string), size (int), class (string)
func RenderHTML(icon string, others ...any) template.HTML {
	size, class := gitea_html.ParseSizeAndClass(defaultSize, "", others...)
	if svgStr, ok := svgIcons[icon]; ok {
		// the code is somewhat hacky, but it just works, because the SVG contents are all normalized
		if size != defaultSize {
			svgStr = strings.Replace(svgStr, fmt.Sprintf(`width="%d"`, defaultSize), fmt.Sprintf(`width="%d"`, size), 1)
			svgStr = strings.Replace(svgStr, fmt.Sprintf(`height="%d"`, defaultSize), fmt.Sprintf(`height="%d"`, size), 1)
		}
		if class != "" {
			svgStr = strings.Replace(svgStr, `class="`, fmt.Sprintf(`class="%s `, class), 1)
		}
		return template.HTML(svgStr)
	}
	return ""
}
