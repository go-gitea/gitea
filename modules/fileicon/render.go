// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon

import (
	"html/template"
	"strings"

	"gitea.dev/modules/setting"
)

type RenderedIconPool struct {
	IconSVGs map[string]template.HTML
}

func NewRenderedIconPool() *RenderedIconPool {
	return &RenderedIconPool{
		IconSVGs: make(map[string]template.HTML),
	}
}

func (p *RenderedIconPool) RenderToHTML() template.HTML {
	if len(p.IconSVGs) == 0 {
		return ""
	}
	sb := &strings.Builder{}
	sb.WriteString(`<div class="svg-icon-container">`)
	for _, icon := range p.IconSVGs {
		sb.WriteString(string(icon))
	}
	sb.WriteString(`</div>`)
	return template.HTML(sb.String())
}

func (p *RenderedIconPool) addSVG(svgID, svgHTML string) {
	if p.IconSVGs[svgID] == "" {
		p.IconSVGs[svgID] = template.HTML(svgHTML)
	}
}

// entryIconTheme returns the configured theme for an entry, folders and symlinks to folders use the folder theme
func entryIconTheme(entry *EntryInfo) string {
	if entry.EntryMode.IsDir() || (entry.EntryMode.IsLink() && entry.SymlinkToMode.IsDir()) {
		return setting.UI.FolderIconTheme
	}
	return setting.UI.FileIconTheme
}

func RenderEntryIconHTML(renderedIconPool *RenderedIconPool, entry *EntryInfo) template.HTML {
	if entryIconTheme(entry) == "material" {
		return DefaultMaterialIconProvider().EntryIconHTML(renderedIconPool, entry)
	}
	return BasicEntryIconHTML(entry)
}

// RenderEntryIconID pools the entry icon and returns its SVG ID and wrapper class for "<use href=#ID>"
func RenderEntryIconID(renderedIconPool *RenderedIconPool, entry *EntryInfo) (svgID, class string) {
	if entryIconTheme(entry) == "material" {
		if svgID, class := DefaultMaterialIconProvider().EntryIconID(renderedIconPool, entry); svgID != "" {
			return svgID, class
		}
	}
	name := BasicEntryIconName(entry)
	svgID = "svg-" + name
	iconHTML := string(RenderEntryIconHTML(nil, entry))
	if !strings.HasPrefix(iconHTML, "<svg ") {
		setting.PanicInDevOrTesting("invalid SVG icon for %s", name) // a missing asset would leave "<use>" pointing at nothing
	}
	renderedIconPool.addSVG(svgID, strings.Replace(iconHTML, "<svg ", `<svg id="`+svgID+`" `, 1))
	return svgID, "svg " + name
}
