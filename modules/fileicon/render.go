// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon

import (
	"html/template"
	"strings"

	"code.gitea.io/gitea/modules/setting"
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

func RenderEntryIconHTML(renderedIconPool *RenderedIconPool, entry *EntryInfo) template.HTML {
	// Use folder theme for directories and symlinks to directories
	theme := setting.UI.FileIconTheme
	if entry.EntryMode.IsDir() || (entry.EntryMode.IsLink() && entry.SymlinkToMode.IsDir()) {
		theme = setting.UI.FolderIconTheme
	}

	if theme == "material" {
		return DefaultMaterialIconProvider().EntryIconHTML(renderedIconPool, entry)
	}
	return BasicEntryIconHTML(entry)
}
