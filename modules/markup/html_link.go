// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"path"

	"code.gitea.io/gitea/modules/util"
)

func ResolveLink(ctx *RenderContext, link, userContentAnchorPrefix string) (result string, resolved bool) {
	isAnchorFragment := link != "" && link[0] == '#'
	if !isAnchorFragment && !IsFullURLString(link) {
		linkBase := ctx.Links.Base
		if ctx.IsWiki {
			if ext := path.Ext(link); ext == "" || ext == ".-" {
				linkBase = ctx.Links.WikiLink() // the link is for a wiki page
			} else if DetectMarkupTypeByFileName(link) != "" {
				linkBase = ctx.Links.WikiLink() // the link is renderable as a wiki page
			} else {
				linkBase = ctx.Links.WikiRawLink() // otherwise, use a raw link instead to view&download medias
			}
		} else if ctx.Links.BranchPath != "" || ctx.Links.TreePath != "" {
			// if there is no BranchPath, then the link will be something like "/owner/repo/src/{the-file-path}"
			// and then this link will be handled by the "legacy-ref" code and be redirected to the default branch like "/owner/repo/src/branch/main/{the-file-path}"
			linkBase = ctx.Links.SrcLink()
		}
		link, resolved = util.URLJoin(linkBase, link), true
	}
	if isAnchorFragment && userContentAnchorPrefix != "" {
		link, resolved = userContentAnchorPrefix+link[1:], true
	}
	return link, resolved
}
