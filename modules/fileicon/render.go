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

func RenderEntryIconWithOpenStatus(renderedIconPool *RenderedIconPool, entry *FileEntry, isOpen bool) template.HTML {
	if setting.UI.FileIconTheme == "material" {
		return DefaultMaterialIconProvider().FileIconWithOpenStatus(renderedIconPool, entry, isOpen)
	}
	return BasicThemeIconWithOpenStatus(entry, isOpen)
}
