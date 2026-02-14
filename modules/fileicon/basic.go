// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon

import (
	"html/template"

	"code.gitea.io/gitea/modules/svg"
	"code.gitea.io/gitea/modules/util"
)

func BasicEntryIconName(entry *EntryInfo) string {
	svgName := "octicon-file"
	switch {
	case entry.EntryMode.IsLink():
		svgName = "octicon-file-symlink-file"
		if entry.SymlinkToMode.IsDir() {
			svgName = "octicon-file-directory-symlink"
		}
	case entry.EntryMode.IsDir():
		svgName = util.Iif(entry.IsOpen, "octicon-file-directory-open-fill", "octicon-file-directory-fill")
	case entry.EntryMode.IsSubModule():
		svgName = "octicon-file-submodule"
	}
	return svgName
}

func BasicEntryIconHTML(entry *EntryInfo) template.HTML {
	return svg.RenderHTML(BasicEntryIconName(entry))
}
