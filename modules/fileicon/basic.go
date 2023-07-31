// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon

import (
	"context"
	"html/template"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/svg"
)

func fileIconBasic(ctx context.Context, entry *git.TreeEntry) template.HTML {
	svgName := "octicon-file"
	switch {
	case entry.IsLink():
		svgName = "octicon-file-symlink-file"
		if te, err := entry.FollowLink(); err == nil && te.IsDir() {
			svgName = "octicon-file-directory-symlink"
		}
	case entry.IsDir():
		svgName = "octicon-file-directory-fill"
	case entry.IsSubModule():
		svgName = "octicon-file-submodule"
	}
	return svg.RenderHTML(svgName)
}

func FileIcon(ctx context.Context, entry *git.TreeEntry) template.HTML {
	if setting.UI.FileIconTheme == "material" {
		return DefaultMaterialIconProvider().FileIcon(ctx, entry)
	}
	return fileIconBasic(ctx, entry)
}
