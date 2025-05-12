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
	sb.WriteString(`<div class=tw-hidden>`)
	for _, icon := range p.IconSVGs {
		sb.WriteString(string(icon))
	}
	sb.WriteString(`</div>`)
	return template.HTML(sb.String())
}

func RenderEntryIconHTML(renderedIconPool *RenderedIconPool, entry *EntryInfo) template.HTML {
	if setting.UI.FileIconTheme == "material" {
		return DefaultMaterialIconProvider().EntryIconHTML(renderedIconPool, entry)
	}
	return BasicEntryIconHTML(entry)
}
