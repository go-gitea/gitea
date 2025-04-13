// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon

import (
	"html/template"
	"strings"

	"code.gitea.io/gitea/modules/git"
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

// TODO: use an interface or struct to replace "*git.TreeEntry", to decouple the fileicon module from git module

func RenderEntryIcon(renderedIconPool *RenderedIconPool, entry *git.TreeEntry) template.HTML {
	if setting.UI.FileIconTheme == "material" {
		return DefaultMaterialIconProvider().FileIcon(renderedIconPool, entry)
	}
	return BasicThemeIcon(entry)
}

func RenderEntryIconOpen(renderedIconPool *RenderedIconPool, entry *git.TreeEntry) template.HTML {
	// TODO: add "open icon" support
	if setting.UI.FileIconTheme == "material" {
		return DefaultMaterialIconProvider().FileIcon(renderedIconPool, entry)
	}
	return BasicThemeIcon(entry)
}
