// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon

import (
	"html/template"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/svg"
)

type FileIcon struct {
	Name      string
	Entry     git.TreeEntry
	EntryMode git.EntryMode
}

func BasicThemeFolderIconName(isOpen bool) string {
	if isOpen {
		return "octicon-file-directory-open-fill"
	}
	return "octicon-file-directory-fill"
}

func BasicThemeFolderIcon(isOpen bool) template.HTML {
	return svg.RenderHTML(BasicThemeFolderIconName(isOpen))
}

func BasicThemeIcon(file *FileIcon) template.HTML {
	svgName := "octicon-file"
	switch {
	case file.EntryMode.IsLink():
		svgName = "octicon-file-symlink-file"
		if te, err := file.Entry.FollowLink(); err == nil && te.IsDir() {
			svgName = "octicon-file-directory-symlink"
		}
	case file.EntryMode.IsDir():
		svgName = BasicThemeFolderIconName(false)
	case file.EntryMode.IsSubModule():
		svgName = "octicon-file-submodule"
	}
	return svg.RenderHTML(svgName)
}
