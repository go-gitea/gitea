// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"code.gitea.io/gitea/modules/util"
)

func ResolveLink(ctx *RenderContext, link, userContentAnchorPrefix string) (result string, resolved bool) {
	isAnchorFragment := link != "" && link[0] == '#'
	if !isAnchorFragment && !IsFullURLString(link) {
		linkBase := ctx.Links.Base
		if ctx.IsWiki {
			// no need to check if the link should be resolved as a wiki link or a wiki raw link
			// just use wiki link here and it will be redirected to a wiki raw link if necessary
			linkBase = ctx.Links.WikiLink()
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
