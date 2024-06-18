// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"code.gitea.io/gitea/modules/markup"

	"github.com/yuin/goldmark/ast"
)

func (g *ASTTransformer) transformLink(ctx *markup.RenderContext, v *ast.Link) {
	if link, resolved := markup.ResolveLink(ctx, string(v.Destination), "#user-content-"); resolved {
		v.Destination = []byte(link)
	}
}
