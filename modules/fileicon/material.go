// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon

import (
	"html/template"
	"path"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/svg"
)

type materialIconRulesData struct {
	FileNames      map[string]string `json:"fileNames"`
	FolderNames    map[string]string `json:"folderNames"`
	FileExtensions map[string]string `json:"fileExtensions"`
	LanguageIDs    map[string]string `json:"languageIds"`
}

type MaterialIconProvider struct {
	once  sync.Once
	rules *materialIconRulesData
	svgs  map[string]string
}

var materialIconProvider MaterialIconProvider

func DefaultMaterialIconProvider() *MaterialIconProvider {
	materialIconProvider.once.Do(materialIconProvider.loadData)
	return &materialIconProvider
}

func (m *MaterialIconProvider) loadData() {
	buf, err := options.AssetFS().ReadFile("fileicon/material-icon-rules.json")
	if err != nil {
		log.Error("Failed to read material icon rules: %v", err)
		return
	}
	err = json.Unmarshal(buf, &m.rules)
	if err != nil {
		log.Error("Failed to unmarshal material icon rules: %v", err)
		return
	}

	buf, err = options.AssetFS().ReadFile("fileicon/material-icon-svgs.json")
	if err != nil {
		log.Error("Failed to read material icon rules: %v", err)
		return
	}
	err = json.Unmarshal(buf, &m.svgs)
	if err != nil {
		log.Error("Failed to unmarshal material icon rules: %v", err)
		return
	}
	log.Debug("Loaded material icon rules and SVG images")
}

func (m *MaterialIconProvider) renderFileIconSVG(ctx reqctx.RequestContext, name, svg, extraClass string) template.HTML {
	data := ctx.GetData()
	renderedSVGs, _ := data["_RenderedSVGs"].(map[string]bool)
	if renderedSVGs == nil {
		renderedSVGs = make(map[string]bool)
		data["_RenderedSVGs"] = renderedSVGs
	}
	// This part is a bit hacky, but it works really well. It should be safe to do so because all SVG icons are generated by us.
	// Will try to refactor this in the future.
	if !strings.HasPrefix(svg, "<svg") {
		panic("Invalid SVG icon")
	}
	svgID := "svg-mfi-" + name
	svgCommonAttrs := `class="svg git-entry-icon ` + extraClass + `" width="16" height="16" aria-hidden="true"`
	posOuterBefore := strings.IndexByte(svg, '>')
	if renderedSVGs[svgID] && posOuterBefore != -1 {
		return template.HTML(`<svg ` + svgCommonAttrs + `><use xlink:href="#` + svgID + `"></use></svg>`)
	}
	svg = `<svg id="` + svgID + `" ` + svgCommonAttrs + svg[4:]
	renderedSVGs[svgID] = true
	return template.HTML(svg)
}

func (m *MaterialIconProvider) FolderIcon(ctx reqctx.RequestContext, isOpen bool) template.HTML {
	// return svg.RenderHTML("material-folder-generic", 16, BasicThemeFolderIconName(isOpen))
	iconName := "folder"
	if isOpen {
		iconName = "folder-open"
	}
	return m.renderIconByName(ctx, iconName, BasicThemeFolderIconName(isOpen))
}

func (m *MaterialIconProvider) FileIcon(ctx reqctx.RequestContext, file *FileIcon) template.HTML {
	if m.rules == nil {
		return BasicThemeIcon(file)
	}

	if file.EntryMode.IsLink() {
		if te, err := file.Entry.FollowLink(); err == nil && te.IsDir() {
			// keep the old "octicon-xxx" class name to make some "theme plugin selector" could still work
			return svg.RenderHTML("material-folder-symlink", 16, "octicon-file-directory-symlink")
		}
		return svg.RenderHTML("octicon-file-symlink-file") // TODO: find some better icons for them
	}

	name := m.findIconNameByGit(file)

	extraClass := "octicon-file"
	switch {
	case file.EntryMode.IsDir():
		extraClass = BasicThemeFolderIconName(false)
	case file.EntryMode.IsSubModule():
		extraClass = "octicon-file-submodule"
	}

	return m.renderIconByName(ctx, name, extraClass)
}

func (m *MaterialIconProvider) renderIconByName(ctx reqctx.RequestContext, name, extraClass string) template.HTML {
	if iconSVG, ok := m.svgs[name]; ok && iconSVG != "" {
		// keep the old "octicon-xxx" class name to make some "theme plugin selector" could still work
		return m.renderFileIconSVG(ctx, name, iconSVG, extraClass)
	}
	return svg.RenderHTML("octicon-file")
}

func (m *MaterialIconProvider) findIconNameWithLangID(s string) string {
	if _, ok := m.svgs[s]; ok {
		return s
	}
	if s, ok := m.rules.LanguageIDs[s]; ok {
		if _, ok = m.svgs[s]; ok {
			return s
		}
	}
	return ""
}

func (m *MaterialIconProvider) FindIconName(name string, isDir bool) string {
	fileNameLower := strings.ToLower(path.Base(name))
	if isDir {
		if s, ok := m.rules.FolderNames[fileNameLower]; ok {
			return s
		}
		return "folder"
	}

	if s, ok := m.rules.FileNames[fileNameLower]; ok {
		if s = m.findIconNameWithLangID(s); s != "" {
			return s
		}
	}

	for i := len(fileNameLower) - 1; i >= 0; i-- {
		if fileNameLower[i] == '.' {
			ext := fileNameLower[i+1:]
			if s, ok := m.rules.FileExtensions[ext]; ok {
				if s = m.findIconNameWithLangID(s); s != "" {
					return s
				}
			}
		}
	}

	return "file"
}

func (m *MaterialIconProvider) findIconNameByGit(file *FileIcon) string {
	if file.EntryMode.IsSubModule() {
		return "folder-git"
	}
	return m.FindIconName(file.Name, file.EntryMode.IsDir())
}
