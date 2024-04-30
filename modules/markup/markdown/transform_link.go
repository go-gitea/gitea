// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"path/filepath"

	"code.gitea.io/gitea/modules/markup"
	giteautil "code.gitea.io/gitea/modules/util"

	"github.com/yuin/goldmark/ast"
)

func (g *ASTTransformer) transformLink(ctx *markup.RenderContext, v *ast.Link) {
	// Links need their href to munged to be a real value
	link := v.Destination
	isAnchorFragment := len(link) > 0 && link[0] == '#'
	if !isAnchorFragment && !markup.IsFullURLBytes(link) {
		base := ctx.Links.Base
		if ctx.IsWiki {
			if filepath.Ext(string(link)) == "" {
				// This link doesn't have a file extension - assume a regular wiki link
				base = ctx.Links.WikiLink()
			} else if markup.Type(string(link)) != "" {
				// If it's a file type we can render, use a regular wiki link
				base = ctx.Links.WikiLink()
			} else {
				// Otherwise, use a raw link instead
				base = ctx.Links.WikiRawLink()
			}
		} else if ctx.Links.HasBranchInfo() {
			base = ctx.Links.SrcLink()
		}
		link = []byte(giteautil.URLJoin(base, string(link)))
	}
	if isAnchorFragment {
		link = []byte("#user-content-" + string(link)[1:])
	}
	v.Destination = link
}
