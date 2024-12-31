// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"code.gitea.io/gitea/modules/markup"

	"github.com/yuin/goldmark/ast"
)

func resolveLink(ctx *markup.RenderContext, link, userContentAnchorPrefix string) (result string, resolved bool) {
	isAnchorFragment := link != "" && link[0] == '#'
	if !isAnchorFragment && !markup.IsFullURLString(link) {
		link, resolved = ctx.RenderHelper.ResolveLink(link, markup.LinkTypeDefault), true
	}
	if isAnchorFragment && userContentAnchorPrefix != "" {
		link, resolved = userContentAnchorPrefix+link[1:], true
	}
	return link, resolved
}

func (g *ASTTransformer) transformLink(ctx *markup.RenderContext, v *ast.Link) {
	if link, resolved := resolveLink(ctx, string(v.Destination), "#user-content-"); resolved {
		v.Destination = []byte(link)
	}
}
