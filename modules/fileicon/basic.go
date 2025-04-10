// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon

import (
	"html/template"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/svg"
)

func BasicThemeFolderIconName(isOpen bool) string {
	if isOpen {
		return "octicon-file-directory-open-fill"
	}
	return "octicon-file-directory-fill"
}

func BasicThemeFolderIconWithOpenStatus(isOpen bool) template.HTML {
	return svg.RenderHTML(BasicThemeFolderIconName(isOpen))
}

func BasicThemeIconWithOpenStatus(entry *git.TreeEntry, isOpen bool) template.HTML {
	// TODO: add "open icon" support
	svgName := "octicon-file"
	switch {
	case entry.IsLink():
		svgName = "octicon-file-symlink-file"
		if te, err := entry.FollowLink(); err == nil && te.IsDir() {
			svgName = "octicon-file-directory-symlink"
		}
	case entry.IsDir():
		svgName = BasicThemeFolderIconName(isOpen)
	case entry.IsSubModule():
		svgName = "octicon-file-submodule"
	}
	return svg.RenderHTML(svgName)
}
